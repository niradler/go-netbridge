package tools

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

var responseChan = make(chan messages.HttpResponseMessage)

type Server struct {
	Conn   *websocket.Conn
	Router *chi.Mux
	Config *config.Config
	Wg     sync.WaitGroup
}

func (s *Server) Start() error {
	log.Println("Server started on :" + s.Config.PORT)
	if err := http.ListenAndServe(":"+s.Config.PORT, s.Router); err != nil {
		log.Fatal("ListenAndServe:", err)
	}

	return nil
}

func (s *Server) WsConnect() {
	serverUrl, err := url.Parse(s.Config.SERVER_URL)
	if err != nil {
		log.Println("Error parsing server url:", err)
		return
	}
	conn, err := tunnel.Connect(*serverUrl)
	if err != nil {
		log.Println("Tunnel connect error:", err)
		return
	}

	s.Conn = conn

	// Increment the WaitGroup counter
	s.Wg.Add(1)

	go func() {
		defer s.Wg.Done() // Decrement the counter when the goroutine completes
		for {
			msg, err := messages.ReadAndParseMessage(conn)
			fmt.Printf("Received message: Type=%s, ID=%s, Total=%d, Chunk=%d, Response=%t\n", msg.Type, msg.ID, msg.Total, msg.Chunk, msg.Response)
			if err != nil {
				log.Printf("Error reading message: %v", err)
				break
			}
			if msg.Type == messages.MessageType.Response {
				responseChan <- *(msg.Params.(*messages.HttpResponseMessage))
			} else {
				err = messages.MessageHandler(conn, msg)
				if err != nil {
					log.Printf("Error handling message: %v", err)
					break
				}
			}
		}
	}()

}

func (s *Server) WsCreate() {
	s.Router.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := tunnel.Create(w, r)
		if err != nil {
			log.Println("Error upgrading connection:", err)
			return
		}

		defer conn.Close()
		s.Conn = conn

		// Increment the WaitGroup counter
		s.Wg.Add(1)

		go func() {
			defer s.Wg.Done() // Decrement the counter when the goroutine completes
			for {
				msg, err := messages.ReadAndParseMessage(conn)
				fmt.Printf("Received message: %+v\n", msg)
				if err != nil {
					log.Printf("Error reading message: %v", err)
					break
				}
				if msg.Type == messages.MessageType.Response {
					fmt.Printf("Received response: %+v\n", msg)
				} else {
					err = messages.MessageHandler(conn, msg)
					if err != nil {
						log.Printf("Error handling message: %v", err)
					}
				}
			}
		}()

		s.Wg.Wait()
	})
}

func (s *Server) Handler(w http.ResponseWriter, r *http.Request) {
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

	u := url.URL{Scheme: s.Config.X_Forwarded_Proto, Host: s.Config.X_Forwarded_Host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}

	msg.Params = messages.HttpRequestMessage{
		Method:  r.Method,
		URL:     u.String(),
		Headers: ConvertHeaders(r.Header),
		Body:    string(bodyBytes),
	}
	fmt.Printf("Request Params: %+v\n", msg.Params)
	err = s.Conn.WriteJSON(msg)
	if err != nil {
		log.Printf("Error writing message: %v", err)
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	select {
	case responseParams := <-responseChan:
		for key, value := range responseParams.Headers {
			w.Header().Set(key, value)
		}
		w.WriteHeader(responseParams.StatusCode)
		w.Write([]byte(responseParams.Body))

	case <-r.Context().Done():
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}

func NewServer(config *config.Config) *Server {
	server := &Server{
		Config: config,
		Router: chi.NewRouter(),
		Wg:     sync.WaitGroup{},
	}

	server.Router.Use(middleware.Logger)

	if config.Type == "server" {
		server.WsCreate()
	} else {
		server.WsConnect()
	}

	server.Router.NotFound(server.Handler)

	return server
}
