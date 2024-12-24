package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/websocket"

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

func convertHeaders(headers http.Header) map[string]string {
	converted := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			converted[key] = values[0]
		}
	}
	return converted
}

var conn *websocket.Conn
var responseChan = make(chan messages.HttpResponseMessage)

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

	err = conn.WriteJSON(messages.Message{
		Type:  messages.MessageType.Ping,
		Total: 1,
		Chunk: 1,
		ID:    messages.CreateId(),
		Params: messages.PingMessage{
			Body: "ping",
		},
		Response: false,
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
	msg.Type = messages.MessageType.Request
	msg.Response = false
	msg.Total = 1
	msg.Chunk = 1
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	msg.Params = messages.HttpRequestMessage{
		Method:  r.Method,
		URL:     "http://localhost:8089",
		Headers: convertHeaders(r.Header),
		Body:    string(bodyBytes),
	}
	err = conn.WriteJSON(msg)
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

func printParamsType(params interface{}) {
	v := reflect.ValueOf(params)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fmt.Printf("Field %s: %s\n", t.Field(i).Name, field.Type())
	}
}
