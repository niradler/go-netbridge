package shared

import (
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
	"github.com/niradler/go-netbridge/messages"
	"github.com/niradler/go-netbridge/tunnel"
)

type HTTPServer struct {
	config *config.Config
	wss    *WebSocketServer
	router *chi.Mux
}

func NewWebSocketServer(hs *HTTPServer) {

	hs.router.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := tunnel.Create(w, r)
		if err != nil {
			log.Println("Error upgrading connection:", err)
			return
		}

		hs.wss = &WebSocketServer{
			conn:         conn,
			responseChan: make(chan messages.HttpResponseMessage),
		}

		defer conn.Close()

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			for {
				msg, err := messages.ReadAndParseMessage(conn)
				fmt.Printf("Received message: %+v %+v\n", msg.Type, msg.ID)
				if err != nil {
					log.Printf("Error reading message: %v", err)
					break
				}
				if msg.Type == messages.MessageType.Response {
					fmt.Printf("Received response: %+v %+v\n", msg.Type, msg.ID)
					hs.wss.responseChan <- *(msg.Params.(*messages.HttpResponseMessage))
				} else {
					err = messages.MessageHandler(conn, msg, *hs.config)
					if err != nil {
						log.Printf("Error handling message: %v", err)
					}
				}
			}
		}()

		wg.Wait()
	})

}

// NewHTTPServer creates a new HTTPServer instance.
func NewHTTPServer(config *config.Config, wss *WebSocketServer) *HTTPServer {
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
	var msg messages.Message
	msg.ID = messages.CreateId()
	msg.Type = messages.MessageType.Request
	msg.Response = false
	msg.Total = 1
	msg.Chunk = 1
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
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
	msg.Params = messages.HttpRequestMessage{
		Method:  r.Method,
		URL:     u.String(),
		Headers: reqHeaders,
		Body:    string(bodyBytes),
	}
	fmt.Printf("Request Params: %+v, %v\n", r.Method, u.String())

	err = hs.wss.SendMessage(msg)
	if err != nil {
		log.Printf("Error writing message: %v", err)
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	select {
	case responseParams := <-hs.wss.responseChan:
		log.Printf("api response: %+v body=%v", responseParams.StatusCode, len(responseParams.Body))
		for key, value := range responseParams.Headers {
			if _, ok := IgnoredHeaders[key]; !ok {
				w.Header().Set(key, strings.Join(value, ","))
			}
		}

		w.WriteHeader(responseParams.StatusCode)
		w.Write([]byte(responseParams.Body))
	case <-r.Context().Done():
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}
