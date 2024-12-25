package shared

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/messages"
	"github.com/niradler/go-netbridge/tunnel"
)

const maxMessageSize = 6 * 1024 * 1024 // 6 MB in bytes

var upgrader = websocket.Upgrader{
	ReadBufferSize:  maxMessageSize,
	WriteBufferSize: maxMessageSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WebSocketServer struct {
	conn         *websocket.Conn
	responseChan chan messages.HttpResponseMessage
	messageMutex sync.Mutex
	messageWG    sync.WaitGroup
}

// NewWebSocketServer initializes a WebSocketServer instance.
func NewWebSocketConnection(cfg *config.Config) (*WebSocketServer, error) {
	wsURL, err := url.Parse(cfg.SERVER_URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebSocket URL: %w", err)
	}
	conn, err := tunnel.Connect(*wsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	server := &WebSocketServer{
		conn:         conn,
		responseChan: make(chan messages.HttpResponseMessage),
	}

	server.messageWG.Add(1)
	go server.listenForMessages()

	return server, nil
}

// listenForMessages processes incoming WebSocket messages.
func (wss *WebSocketServer) listenForMessages() {
	defer wss.messageWG.Done()
	for {
		msg, err := messages.ReadAndParseMessage(wss.conn)
		if err != nil {
			log.Printf("Error reading message: %v", err)
			break
		}
		if msg.Type == messages.MessageType.Response {
			wss.responseChan <- *(msg.Params.(*messages.HttpResponseMessage))
		} else {
			err = messages.MessageHandler(wss.conn, msg)
			if err != nil {
				log.Printf("Error handling message: %v", err)
				break
			}
		}
	}
}

// Close cleans up WebSocket resources.
func (wss *WebSocketServer) Close() {
	wss.conn.Close()
	wss.messageWG.Wait()
}

// SendMessage sends a message over the WebSocket connection.
func (wss *WebSocketServer) SendMessage(msg messages.Message) error {
	wss.messageMutex.Lock()
	defer wss.messageMutex.Unlock()
	return wss.conn.WriteJSON(msg)
}

// HTTPServer is a wrapper for the HTTP server with WebSocket integration.
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
				fmt.Printf("Received message: %+v\n", msg)
				if err != nil {
					log.Printf("Error reading message: %v", err)
					break
				}
				if msg.Type == messages.MessageType.Response {
					fmt.Printf("Received response: %+v\n", msg)
					hs.wss.responseChan <- *(msg.Params.(*messages.HttpResponseMessage))
				} else {
					err = messages.MessageHandler(conn, msg)
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

	hs := &HTTPServer{
		wss:    wss,
		router: router,
		config: config,
	}

	router.NotFound(hs.proxyHandler)

	return hs
}

// Start starts the HTTP server on the specified port.
func (hs *HTTPServer) Start(port string) error {
	return http.ListenAndServe(port, hs.router)
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

	u := url.URL{Scheme: hs.config.X_Forwarded_Proto, Host: hs.config.X_Forwarded_Host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}

	msg.Params = messages.HttpRequestMessage{
		Method:  r.Method,
		URL:     u.String(),
		Headers: ConvertHeaders(r.Header),
		Body:    string(bodyBytes),
	}
	fmt.Printf("Request Params: %+v\n", msg.Params)

	err = hs.wss.SendMessage(msg)
	if err != nil {
		log.Printf("Error writing message: %v", err)
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	select {
	case responseParams := <-hs.wss.responseChan:
		for key, value := range responseParams.Headers {
			w.Header().Set(key, value)
		}
		w.WriteHeader(responseParams.StatusCode)
		w.Write([]byte(responseParams.Body))
	case <-r.Context().Done():
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}
