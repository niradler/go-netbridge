package shared

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/tunnel"
	"github.com/niradler/socketflow"
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
	Client       *socketflow.WebSocketClient
	responseChan chan HttpResponseMessage
	messageMutex sync.Mutex
	messageWG    sync.WaitGroup
	config       *config.Config
}

func (wss *WebSocketServer) Close() {
	wss.Client.Close()
	wss.messageWG.Wait()
}

func (wss *WebSocketServer) SendMessage(msg socketflow.Message) error {
	wss.messageMutex.Lock()
	defer wss.messageMutex.Unlock()
	_, err := wss.Client.SendMessage(msg.Topic, msg.Payload)
	return err
}

func NewWebSocketConnection(cfg *config.Config) (*WebSocketServer, error) {
	wsURL, err := url.Parse(cfg.SERVER_URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebSocket URL: %w", err)
	}
	client, err := tunnel.Connect(*wsURL, *cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	fmt.Println("WebSocket connected")

	server := &WebSocketServer{
		Client:       client,
		responseChan: make(chan HttpResponseMessage),
		config:       cfg,
	}

	server.messageWG.Add(1)
	go client.ReceiveMessages()

	go func() {
		for msg := range client.Subscribe("request") {
			fmt.Printf("Received message: %+v\n", string(msg.Payload))
			var req HttpRequestMessage
			err := json.Unmarshal(msg.Payload, &req)
			if err != nil {
				log.Printf("Error parsing request message: %v", err)
				continue
			}
			err = HttpRequestResponse(&req, cfg, client)
			if err != nil {
				log.Printf("Error in http request: %v", err)
				continue
			}
		}
	}()

	statusChan := client.SubscribeToStatus()
	go func() {
		for status := range statusChan {
			log.Printf("Received status: %v\n", status)
		}
	}()

	return server, nil
}
