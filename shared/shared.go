package shared

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/socketflow"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
)

type HttpRequestMessage struct {
	RequestID string      `json:"requestId"`
	Method    string      `json:"method"`
	URL       string      `json:"url"`
	Headers   http.Header `json:"headers"`
	Body      []byte      `json:"body"`
}

type HttpResponseMessage struct {
	RequestID  string      `json:"requestId"`
	StatusCode int         `json:"statusCode"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body"`
}

var ignoredHeaders = map[string]struct{}{
	"Connection":               {},
	"Keep-Alive":               {},
	"Proxy-Authenticate":       {},
	"Proxy-Authorization":      {},
	"Te":                       {},
	"Trailer":                  {},
	"Transfer-Encoding":        {},
	"Upgrade":                  {},
	"Content-Length":           {},
	"Host":                     {},
	"X-Auth-Secret":            {},
	"Sec-Websocket-Accept":     {},
	"Sec-Websocket-Extensions": {},
	"Sec-Websocket-Key":        {},
	"Sec-Websocket-Protocol":   {},
	"Sec-Websocket-Version":    {},
}

func HttpRequest(requestParams *HttpRequestMessage, cfg *config.Config) (*HttpResponseMessage, error) {
	logger := GetLogger()
	parsed, err := url.Parse(requestParams.URL)
	if err != nil {
		return nil, errNotAllowed
	}
	if err := HostAllowed(parsed, cfg); err != nil {
		return nil, err
	}
	if !validMethod(requestParams.Method) {
		return nil, errNotAllowed
	}
	logger.Info("upstream request",
		zap.String("method", requestParams.Method),
		zap.String("host", parsed.Hostname()),
		zap.Int("body_bytes", len(requestParams.Body)),
	)

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(parsed.String())
	req.Header.SetMethod(requestParams.Method)
	for key, values := range sanitizeHeaders(requestParams.Headers) {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.SetBodyRaw(requestParams.Body)

	client := &fasthttp.Client{
		ReadTimeout:              30 * time.Second,
		WriteTimeout:             30 * time.Second,
		MaxIdleConnDuration:      60 * time.Second,
		MaxResponseBodySize:      maxBodyBytes,
		NoDefaultUserAgentHeader: true,
		TLSConfig:                upstreamTLS(cfg),
	}
	if err := client.Do(req, resp); err != nil {
		logger.Error("upstream request failed", zap.String("host", parsed.Hostname()), zap.Error(err))
		return nil, err
	}

	headers := make(http.Header)
	resp.Header.VisitAll(func(key, value []byte) {
		name := string(key)
		if _, skip := ignoredHeaders[http.CanonicalHeaderKey(name)]; skip {
			return
		}
		headers.Add(name, string(value))
	})
	return &HttpResponseMessage{
		RequestID:  requestParams.RequestID,
		StatusCode: resp.StatusCode(),
		Headers:    sanitizeHeaders(headers),
		Body:       append([]byte(nil), resp.Body()...),
	}, nil
}

func HttpRequestResponse(requestParams *HttpRequestMessage, cfg *config.Config, client *socketflow.WebSocketClient) error {
	response, err := HttpRequest(requestParams, cfg)
	if err != nil {
		response = &HttpResponseMessage{
			RequestID:  requestParams.RequestID,
			StatusCode: http.StatusBadGateway,
			Headers:    make(http.Header),
			Body:       []byte("Bad Gateway"),
		}
	}
	return SendResponseMessage(*response, client)
}

func SendResponseMessage(message HttpResponseMessage, client *socketflow.WebSocketClient) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return err
	}
	_, err = client.SendMessage("response", payload)
	return err
}

func sanitizeHeaders(headers http.Header) http.Header {
	clean := make(http.Header)
	for key, values := range headers {
		if _, skip := ignoredHeaders[http.CanonicalHeaderKey(key)]; skip {
			continue
		}
		for _, value := range values {
			clean.Add(key, value)
		}
	}
	return clean
}

func upstreamTLS(cfg *config.Config) *tls.Config {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if cfg.REQUEST_CA_FILE != "" {
		pem, err := os.ReadFile(cfg.REQUEST_CA_FILE)
		if err != nil {
			GetLogger().Error("failed to read CA file")
			return tlsConfig
		}
		pool := x509.NewCertPool()
		if pool.AppendCertsFromPEM(pem) {
			tlsConfig.RootCAs = pool
		}
	}
	if cfg.INSECURE_SKIP_VERIFY {
		GetLogger().Warn("TLS verification of upstream requests is disabled")
		tlsConfig.InsecureSkipVerify = true
	}
	return tlsConfig
}
