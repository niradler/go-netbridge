package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

	"github.com/niradler/go-netbridge/messages"
	"github.com/niradler/go-netbridge/tunnel"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var conn *websocket.Conn
var responseChan = make(chan messages.Message)

func main() {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	var err error
	conn, err = tunnel.Connect(u)
	if err != nil {
		log.Fatalf("Failed to connect to WebSocket: %v", err)
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
				fmt.Printf("Received response: %+v\n", msg.Response)
				responseChan <- msg
			} else {
				err = messages.MessageHandler(conn, msg)
				if err != nil {
					log.Printf("Error handling message: %v", err)
					break
				}
			}
		}
	}()

	err = conn.WriteJSON(messages.Message{
		Type:  messages.MessageType.Ping,
		Total: 1,
		Chunk: 1,
		ID:    messages.CreateId(),
		Params: messages.PingMessage{
			Body: "ping",
		},
	})

	if err != nil {
		log.Printf("Error writing message: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Post("/send", sendMessageHandler)

	http.ListenAndServe(":8081", r)

	wg.Wait()
}

func sendMessageHandler(w http.ResponseWriter, r *http.Request) {
	var msg messages.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	msg.ID = messages.CreateId()
	err := conn.WriteJSON(msg)
	if err != nil {
		log.Printf("Error writing message: %v", err)
		http.Error(w, "Failed to send message", http.StatusInternalServerError)
		return
	}

	select {
	case responseMsg := <-responseChan:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(responseMsg.Response)
	case <-r.Context().Done():
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}
