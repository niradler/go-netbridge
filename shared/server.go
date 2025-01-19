package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/tunnel"
	"github.com/niradler/socketflow"
)

type HTTPServer struct {
	config *config.Config
	wss    *socketflow.WebSocketClient
	router *chi.Mux
}

func NewWebSocketServer(hs *HTTPServer) {
	hs.router.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		client, err := tunnel.Create(w, r)
		if err != nil {
			log.Println("Error upgrading connection:", err)
			return
		}
		log.Println("WebSocket connection established")
		hs.wss = client

		defer client.Close()

		go client.ReceiveMessages()

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			for msg := range client.Subscribe("request") {
				fmt.Printf("Received message: %+v\n", string(msg.Payload))
				var req HttpRequestMessage
				err := json.Unmarshal(msg.Payload, &req)
				if err != nil {
					log.Printf("Error parsing request message: %v", err)
					continue
				}
				err = HttpRequestResponse(&req, hs.config, client)
				if err != nil {
					log.Printf("Error in http request: %v", err)
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

	router.NotFound(hs.proxyHandler)

	return hs
}

// Start starts the HTTP server on the specified port.
func (hs *HTTPServer) Start() error {
	if hs.config.SSL_CERT_FILE != "" && hs.config.SSL_KEY_FILE != "" {
		return http.ListenAndServeTLS(":"+hs.config.PORT, hs.config.SSL_CERT_FILE, hs.config.SSL_KEY_FILE, hs.router)
	}

	return http.ListenAndServe("localhost:"+hs.config.PORT, hs.router)
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
	"Content-Encoding":         {},
}

func (hs *HTTPServer) proxyHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received request: %s %s", r.Method, r.URL.String())
	// Stream the request body in chunks
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
	u := url.URL{Scheme: proto, Host: host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}

	reqHeaders := r.Header.Clone()
	reqHeaders.Del("X-Forwarded-Proto")
	reqHeaders.Del("X-Forwarded-Host")
	reqHeaders.Del("X-Auth-SECRET")
	req := HttpRequestMessage{
		Method:  r.Method,
		URL:     u.String(),
		Headers: reqHeaders,
		Body:    payload,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		http.Error(w, "Failed to marshal request", http.StatusInternalServerError)
		return
	}

	id, err := hs.wss.SendMessage("request", payload)
	if err != nil {
		log.Printf("Error writing message: %v", err)
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}
	fmt.Println("Sent message with ID:", id)

	select {
	case responseMessage := <-hs.wss.Subscribe("response"):
		log.Printf("Response message: %+v, %t", responseMessage.ID, responseMessage.IsChunk)
		var response HttpResponseMessage
		err := json.Unmarshal(responseMessage.Payload, &response)
		if err != nil {
			log.Printf("Error unmarshalling response: %v", err)
			http.Error(w, "Failed to parse response", http.StatusInternalServerError)
			return
		}
		for key, value := range response.Headers {
			if _, ok := IgnoredHeaders[key]; !ok {
				w.Header().Set(key, strings.Join(value, ","))
			}
		}

		w.WriteHeader(response.StatusCode)

		_, err = w.Write(response.Body)
		if err != nil {
			log.Printf("Error writing chunk to response: %v", err)
			return
		}
		w.(http.Flusher).Flush() // Flush the response to the client
	case <-r.Context().Done():
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}
