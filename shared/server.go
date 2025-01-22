package shared

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/tunnel"
	"github.com/niradler/socketflow"
	"go.uber.org/zap"
)

type HTTPServer struct {
	config *config.Config
	wss    *socketflow.WebSocketClient
	router *chi.Mux
}

func NewWebSocketServer(hs *HTTPServer) {
	logger := GetLogger()
	hs.router.Get("/_ws", func(w http.ResponseWriter, r *http.Request) {
		client, err := tunnel.Create(w, r)
		if err != nil {
			logger.Error("Error upgrading connection", zap.String("error", err.Error()))
			return
		}
		logger.Info("WebSocket connection established")
		hs.wss = client

		defer client.Close()

		go client.ReceiveMessages()

		statusChan := client.SubscribeToStatus()
		go func() {
			for status := range statusChan {
				logger.Debug("Received status", zap.Any("status", status))
			}
		}()

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			for msg := range client.Subscribe("request") {
				logger.Debug("Received message", zap.String("message", string(msg.Payload)))
				var req HttpRequestMessage
				err := json.Unmarshal(msg.Payload, &req)
				if err != nil {
					logger.Error("Error parsing request message", zap.String("error", err.Error()))
					continue
				}
				err = HttpRequestResponse(&req, hs.config, client)
				if err != nil {
					logger.Error("Error in http request", zap.String("error", err.Error()))
					continue
				}
			}
		}()

		wg.Wait()

	})
}

// NewHTTPServer creates a new HTTPServer instance.
func NewHTTPServer(config *config.Config, wss *socketflow.WebSocketClient) *HTTPServer {
	router := chi.NewRouter()
	router.Use(middleware.Logger)

	if len(config.WHITE_LIST) > 0 {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				allowed := false
				for _, listed := range config.WHITE_LIST {
					if strings.HasPrefix(r.RemoteAddr, listed) || strings.HasSuffix(r.Host, listed) {
						allowed = true
						break
					}
				}
				if !allowed {
					logger.Error("Blocked list", zap.String("remoteAddr", r.RemoteAddr), zap.String("host", r.Host))
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
	}

	if config.SECRET != "" && config.Type != "client" {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authHeader := r.Header.Get("X-Auth-SECRET")
				if authHeader != config.SECRET {
					http.Error(w, "Forbidden", http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
			})
		})
	}
	hs := &HTTPServer{
		wss:    wss,
		router: router,
		config: config,
	}

	router.Get("/_health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
		return
	})

	router.NotFound(hs.proxyHandler)

	return hs
}

// Start starts the HTTP server on the specified port.
func (hs *HTTPServer) Start() error {
	logger := GetLogger()
	if hs.config.SSL_CERT_FILE != "" && hs.config.SSL_KEY_FILE != "" {
		logger.Debug("Starting HTTPS server", zap.String("port", hs.config.PORT))
		return http.ListenAndServeTLS(":"+hs.config.PORT, hs.config.SSL_CERT_FILE, hs.config.SSL_KEY_FILE, hs.router)
	}

	logger.Info("Starting HTTP server", zap.String("port", hs.config.PORT))
	return http.ListenAndServe(":"+hs.config.PORT, hs.router)
}

var IgnoredHeaders = map[string]struct{}{
	"Content-Length":           {},
	"Transfer-Encoding":        {},
	"Connection":               {},
	"Keep-Alive":               {},
	"Proxy-Authenticate":       {},
	"Proxy-Authorization":      {},
	"TE":                       {},
	"Trailer":                  {},
	"Upgrade":                  {},
	"Sec-WebSocket-Accept":     {},
	"Sec-WebSocket-Extensions": {},
	"Sec-WebSocket-Key":        {},
	"Sec-WebSocket-Protocol":   {},
	"Sec-WebSocket-Version":    {},
}

func getResHeaders(headers http.Header) http.Header {
	cleanHeaders := make(http.Header)
	for key, value := range headers {
		if _, ok := IgnoredHeaders[key]; !ok {
			cleanHeaders[key] = value
		}
	}
	return cleanHeaders
}

func getReqHeaders(headers http.Header) http.Header {
	reqHeaders := headers.Clone()
	reqHeaders.Del("X-Forwarded-Proto")
	reqHeaders.Del("X-Forwarded-Host")
	reqHeaders.Del("X-Auth-SECRET")
	reqHeaders.Del("X-Proxy-Type")

	return reqHeaders
}

func (hs *HTTPServer) proxyRequest(w http.ResponseWriter, r *http.Request, req HttpRequestMessage) {
	logger.Debug("Received request", zap.String("method", r.Method), zap.String("url", r.URL.String()))
	res, err := HttpRequest(&req, hs.config)
	if err != nil {
		logger.Error("Error HttpRequest", zap.Error(err))
		http.Error(w, "Failed to do request", http.StatusInternalServerError)
		return
	}
	for key, value := range getResHeaders(res.Headers) {
		w.Header().Set(key, strings.Join(value, ","))
	}

	w.WriteHeader(res.StatusCode)
	_, err = w.Write(res.Body)
	if err != nil {
		logger.Error("Error Write", zap.Error(err))
		http.Error(w, "Failed to Write Response", http.StatusInternalServerError)
		return
	}
	// io.Copy(w, res.Body)
}

func (hs *HTTPServer) proxyHandler(w http.ResponseWriter, r *http.Request) {
	logger.Debug("Received request", zap.String("method", r.Method), zap.String("url", r.URL.String()))

	if hs == nil {
		logger.Error("HTTPServer instance is nil")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// TODO: handle large request bodies, use config for chunk size
	chunkSize := 1024 * 1024 // 1 MB chunks
	buf := make([]byte, chunkSize)
	var payload []byte

	for {
		n, err := r.Body.Read(buf)
		if err != nil && err != io.EOF {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		if n == 0 {
			break
		}
		payload = append(payload, buf[:n]...)
	}

	host := hs.config.X_Forwarded_Host
	if r.Header.Get("X-Forwarded-Host") != "" {
		host = r.Header.Get("X-Forwarded-Host")
	}

	proto := hs.config.X_Forwarded_Proto
	if r.Header.Get("X-Forwarded-Proto") != "" {
		proto = r.Header.Get("X-Forwarded-Proto")
	}

	proxyType := hs.config.PROXY_TYPE
	if r.Header.Get("X-Proxy-Type") != "" {
		proxyType = r.Header.Get("X-Proxy-Type")
	}

	if proxyType == "" || proto == "" || host == "" {
		logger.Error("Missing headers")
		http.Error(w, "Missing headers", http.StatusBadRequest)
		return
	}

	logger.Debug("Proxy type", zap.String("type", proxyType), zap.String("proto", proto), zap.String("host", host))

	if proxyType == "server" {
		serverUrl, err := url.Parse(hs.config.SERVER_URL)
		if err != nil {
			logger.Error("Error Parse url", zap.String("error", err.Error()))
			http.Error(w, "Error Parse url", http.StatusInternalServerError)
			return
		}
		serverUrl.Path = r.URL.Path
		serverUrl.RawQuery = r.URL.RawQuery
		reqHeaders := r.Header.Clone()
		reqHeaders.Set("x-Proxy-Type", "proxy")
		reqHeaders.Set("X-Forwarded-Host", host)
		reqHeaders.Set("X-Forwarded-Proto", proto)
		hs.proxyRequest(w, r, HttpRequestMessage{
			Method:  r.Method,
			URL:     serverUrl.String(),
			Headers: reqHeaders,
			Body:    payload,
		})
	} else if proxyType == "proxy" {
		hostUrl := url.URL{Scheme: r.Header.Get("X-Forwarded-Proto"), Host: r.Header.Get("X-Forwarded-Host"), Path: r.URL.Path, RawQuery: r.URL.RawQuery}
		hs.proxyRequest(w, r, HttpRequestMessage{
			Method:  r.Method,
			URL:     hostUrl.String(),
			Headers: getReqHeaders(r.Header),
			Body:    payload,
		})
	} else {
		u := url.URL{Scheme: proto, Host: host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}
		reqMsg := HttpRequestMessage{
			Method:  r.Method,
			URL:     u.String(),
			Headers: getReqHeaders(r.Header),
			Body:    payload,
		}
		payload, err := json.Marshal(reqMsg)
		if err != nil {
			logger.Error("Error marshaling request", zap.String("error", err.Error()))
			http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
			return
		}

		id, err := hs.wss.SendMessage("request", payload)
		if err != nil {
			logger.Error("Error writing message", zap.String("error", err.Error()))
			http.Error(w, "Failed to send message", http.StatusInternalServerError)
			return
		}
		logger.Debug("Sent message with ID", zap.String("id", id))

		select {
		case responseMessage := <-hs.wss.Subscribe("response"):
			logger.Debug("Response message", zap.String("id", responseMessage.ID), zap.Bool("isChunk", responseMessage.IsChunk))
			var response HttpResponseMessage
			err := json.Unmarshal(responseMessage.Payload, &response)
			if err != nil {
				logger.Error("Error unmarshalling response", zap.String("error", err.Error()))
				http.Error(w, "Failed to parse response", http.StatusInternalServerError)
				return
			}
			for key, value := range getResHeaders(response.Headers) {
				w.Header().Set(key, strings.Join(value, ","))
			}

			w.WriteHeader(response.StatusCode)

			_, err = w.Write(response.Body)
			if err != nil {
				logger.Error("Error writing chunk to response", zap.String("error", err.Error()))
				return
			}
			w.(http.Flusher).Flush() // Flush the response to the client
		case <-r.Context().Done():
			http.Error(w, "Request timed out", http.StatusGatewayTimeout)
		}
	}

}
