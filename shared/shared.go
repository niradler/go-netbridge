package shared

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/socketflow"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

func PrintTypes(s interface{}) {
	v := reflect.ValueOf(s)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fmt.Printf("Field %s: %s\n", t.Field(i).Name, field.Type())
	}
}

func ConvertHeadersMulti(headers http.Header) map[string]string {
	converted := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			converted[key] = values[0]
		}
	}
	return converted
}

func Ping(client *socketflow.WebSocketClient) error {
	for i := 1; i <= 3; i++ {
		_, err := client.SendMessage("ping", []byte("ping"))
		if err != nil {
			return err
		}
	}
	return nil
}

type HttpRequestMessage struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
	Body    []byte              `json:"body"`
}

type HttpResponseMessage struct {
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte              `json:"body"`
}

type HttpResponse struct {
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers"`
	Body       []byte
}

func RequestAllowed(requestParams *HttpRequestMessage, config *config.Config) error {
	if len(config.WHITE_LIST) > 0 {
		host, _ := ExtractHostname(requestParams.URL)
		logger.Info("WHITE_LIST", zap.String("host", host))
		allowed := false
		for _, listed := range config.WHITE_LIST {
			if strings.HasPrefix(host, listed) {
				allowed = true
				break
			}
		}

		if !allowed {
			return fmt.Errorf("Request not allowed for host: %s", host)

		}

	}
	return nil
}

func ExtractHostname(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}
	return parsedURL.Host, nil
}

func HttpRequest(requestParams *HttpRequestMessage, config *config.Config) (*HttpResponseMessage, error) {
	logger := GetLogger()
	logger.Info("HttpRequest", zap.String("Method", requestParams.Method), zap.String("URL", requestParams.URL), zap.Int("BodyLen", len(requestParams.Body)))

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(requestParams.URL)
	req.Header.SetMethod(requestParams.Method)
	for key, value := range requestParams.Headers {
		for _, v := range value {
			req.Header.Add(key, v)
		}
	}
	req.SetBodyRaw(requestParams.Body)

	client := &fasthttp.Client{
		ReadBufferSize:      16 * 1024,
		WriteBufferSize:     16 * 1024,
		MaxConnDuration:     30 * time.Minute,
		MaxIdleConnDuration: 60 * time.Second,
		ReadTimeout:         30 * time.Second, // Increased timeout
		WriteTimeout:        30 * time.Second, // Increased timeout
	}

	if config.REQUEST_CA_FILE != "" {
		caCert, err := os.ReadFile(config.REQUEST_CA_FILE)
		if err != nil {
			logger.Error("Error reading CA file", zap.String("error", err.Error()))
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			logger.Error("Failed to append CA certs")
			return nil, fmt.Errorf("failed to append CA certs")
		}
		client.TLSConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	// Retry logic
	maxRetries := 2
	var err error
	for i := 0; i < maxRetries; i++ {
		logger.Debug("DoRedirects", zap.Int("attempt", i+1))
		err = client.DoRedirects(req, resp, 10)
		if err == nil {
			break // Success, exit retry loop
		}
		logger.Warn("Retrying request", zap.Int("attempt", i+1), zap.String("error", err.Error()))
		time.Sleep(500 * time.Millisecond) // Add a delay between retries
	}
	if err != nil {
		logger.Error("Error in request after retries", zap.String("error", err.Error()))
		return nil, err
	}

	headers := make(map[string][]string)
	resp.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = append(headers[string(key)], string(value))
	})

	logger.Debug("HttpResponse", zap.Int("StatusCode", resp.StatusCode()), zap.String("Method", requestParams.Method), zap.String("URL", requestParams.URL))

	return &HttpResponseMessage{
		StatusCode: resp.StatusCode(),
		Headers:    headers,
		Body:       resp.Body(), // Streamed body
	}, nil
}

func HttpRequestResponse(requestParams *HttpRequestMessage, config *config.Config, wss *socketflow.WebSocketClient) error {
	logger := GetLogger()
	logger.Info("HttpRequestMessage", zap.String("Method", requestParams.Method), zap.String("URL", requestParams.URL), zap.Int("BodyLen", len(requestParams.Body)))

	if err := RequestAllowed(requestParams, config); err != nil {
		return err
	}

	res, err := HttpRequest(requestParams, config)
	if err != nil {
		return err
	}

	return SendResponseMessage(*res, wss)
}

func SendResponseMessage(resMsg HttpResponseMessage, wss *socketflow.WebSocketClient) error {
	logger.Debug("SendResponse", zap.Int("StatusCode", resMsg.StatusCode))

	payload, err := json.Marshal(resMsg)
	if err != nil {
		logger.Error("Error marshaling response", zap.String("error", err.Error()))
		return err
	}

	id, err := wss.SendMessage("response", payload)
	if err != nil {
		logger.Error("Error in send response", zap.String("error", err.Error()))
		return err
	}

	logger.Debug("Sent response", zap.String("ID", id))
	return nil
}
