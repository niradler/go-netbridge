package tunnel

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/socketflow"
)

const maxMessageSize = 6 * 1024 * 1024 // 6 MB in bytes

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  maxMessageSize,
	WriteBufferSize: maxMessageSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Create(w http.ResponseWriter, r *http.Request) (*socketflow.WebSocketClient, error) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return nil, err
	}
	client := socketflow.NewWebSocketClient(conn, socketflow.Config{
		ChunkSize:        1024,
		ReassembleChunks: true,
	})
	return client, nil
}

func Connect(url url.URL, config config.Config) (*socketflow.WebSocketClient, error) {
	headers := http.Header{}
	if config.SECRET != "" && config.Type == "client" {
		headers.Add("Authorization", config.SECRET)
	}

	var client *socketflow.WebSocketClient
	var err error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		conn, _, err := websocket.DefaultDialer.Dial(url.String(), headers)
		if err == nil {
			client = socketflow.NewWebSocketClient(conn, socketflow.Config{
				ChunkSize:        1024, //TODO: chunk should be bigger
				ReassembleChunks: true, // TODO: use chunk to stream body instead
			})
			return client, nil
		}
		time.Sleep(1 * time.Second)
		log.Printf("Failed to connect to WebSocket server (attempt %d/%d): %v", i+1, maxRetries, err)
	}

	return nil, err
}
