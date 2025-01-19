package shared

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/socketflow"
	"github.com/valyala/fasthttp"
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

func HttpRequestResponse(requestParams *HttpRequestMessage, config *config.Config, wss *socketflow.WebSocketClient) error {
	log.Printf("HttpRequestMessage: %v  %v bodylen=%v", requestParams.Method, requestParams.URL, len(requestParams.Body))

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
	}

	if config.REQUEST_CA_FILE != "" {
		caCert, err := os.ReadFile(config.REQUEST_CA_FILE)
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		client.TLSConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	err := client.DoRedirects(req, resp, 10)
	if err != nil {
		return err
	}

	var body []byte
	if resp.Header.Peek("Content-Encoding") != nil && string(resp.Header.Peek("Content-Encoding")) == "gzip" {
		bodyBytes, err := resp.BodyGunzip()
		if err != nil {
			return err
		}
		body = bodyBytes
	} else {
		body = resp.Body()
		// resp.StreamBody
	}

	headers := make(map[string][]string)
	resp.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = append(headers[string(key)], string(value))
	})

	log.Printf("HttpResponseMessage, StatusCode=%v Method=%v URL=%v", resp.StatusCode(), requestParams.Method, requestParams.URL)
	res := HttpResponseMessage{
		StatusCode: resp.StatusCode(),
		Headers:    headers,
		Body:       body,
	}
	payload, err := json.Marshal(res)
	id, err := wss.SendMessage("response", payload)
	if err != nil {
		log.Printf("Error in send response: %v", err)
		return err
	}

	fmt.Println("Sent response with ID:", id)

	return nil
}
