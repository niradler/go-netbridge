package shared

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/socketflow"
	"go.uber.org/zap"
)

type HTTPServer struct {
	config  *config.Config
	router  *chi.Mux
	pending *Pending
	mu      sync.Mutex
	client  *socketflow.WebSocketClient
	cancel  func()
}

func NewHTTPServer(cfg *config.Config, client *socketflow.WebSocketClient) *HTTPServer {
	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	server := &HTTPServer{
		config:  cfg,
		router:  router,
		pending: NewPending(),
	}
	if client != nil {
		server.client = client
	}
	router.Get("/_health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	router.NotFound(server.proxyHandler)
	return server
}

func (hs *HTTPServer) Start() error {
	logger := GetLogger()
	if hs.config.INSECURE_SKIP_VERIFY {
		logger.Warn("INSECURE_SKIP_VERIFY is enabled")
	}
	server := &http.Server{
		Addr:              net.JoinHostPort(hs.config.BIND_ADDR, hs.config.PORT),
		Handler:           hs.router,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	if hs.config.SSL_CERT_FILE != "" && hs.config.SSL_KEY_FILE != "" {
		cert, err := tls.LoadX509KeyPair(hs.config.SSL_CERT_FILE, hs.config.SSL_KEY_FILE)
		if err != nil {
			return err
		}
		server.TLSConfig = &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
		logger.Info("starting https server", zap.String("addr", server.Addr))
		return server.ListenAndServeTLS("", "")
	}
	logger.Info("starting http server", zap.String("addr", server.Addr))
	return server.ListenAndServe()
}

func (hs *HTTPServer) proxyHandler(w http.ResponseWriter, r *http.Request) {
	if !validMethod(r.Method) {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	body, err := readLimited(r.Body, maxBodyBytes)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, errBodyTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		http.Error(w, http.StatusText(status), status)
		return
	}

	target, err := RequestTarget(r, hs.config)
	if err != nil || HostAllowed(target, hs.config) != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	message := HttpRequestMessage{
		RequestID: newRequestID(),
		Method:    r.Method,
		URL:       target.String(),
		Headers:   sanitizeHeaders(r.Header),
		Body:      body,
	}

	if hs.config.PROXY_TYPE == "server" || hs.config.PROXY_TYPE == "proxy" {
		response, err := HttpRequest(&message, hs.config)
		if err != nil {
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
			return
		}
		writeResponse(w, response)
		return
	}

	client := hs.currentClient()
	if client == nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	payload, err := json.Marshal(message)
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	if _, err := client.SendMessage("request", payload); err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	response, err := hs.pending.Wait(message.RequestID, r.Context(), 30*time.Second)
	if err != nil {
		http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
		return
	}
	writeResponse(w, &response)
}

func writeResponse(w http.ResponseWriter, response *HttpResponseMessage) {
	for key, values := range sanitizeHeaders(response.Headers) {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	status := response.StatusCode
	if status < 100 || status > 599 {
		status = http.StatusBadGateway
	}
	w.WriteHeader(status)
	_, _ = w.Write(response.Body)
}

func (hs *HTTPServer) currentClient() *socketflow.WebSocketClient {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	return hs.client
}

func (hs *HTTPServer) setClient(client *socketflow.WebSocketClient, cancel func()) {
	hs.mu.Lock()
	previousCancel := hs.cancel
	previous := hs.client
	hs.client = client
	hs.cancel = cancel
	hs.mu.Unlock()
	if previous != nil {
		_ = previous.Close()
	}
	if previousCancel != nil {
		previousCancel()
	}
}

var errBodyTooLarge = errors.New("body too large")

func readLimited(body io.Reader, limit int64) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(payload)) > limit {
		return nil, errBodyTooLarge
	}
	return payload, nil
}

func newRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf)
}
