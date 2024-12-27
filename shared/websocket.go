package shared

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"

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
	config       *config.Config
}

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
			err = messages.MessageHandler(wss.conn, msg, *wss.config)
			if err != nil {
				log.Printf("Error handling message: %v", err)
				break
			}
		}
	}
}

func (wss *WebSocketServer) Close() {
	wss.conn.Close()
	wss.messageWG.Wait()
}

func (wss *WebSocketServer) SendMessage(msg messages.Message) error {
	wss.messageMutex.Lock()
	defer wss.messageMutex.Unlock()
	return wss.conn.WriteJSON(msg)
}

func NewWebSocketConnection(cfg *config.Config) (*WebSocketServer, error) {
	wsURL, err := url.Parse(cfg.SERVER_URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse WebSocket URL: %w", err)
	}
	conn, err := tunnel.Connect(*wsURL, *cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	Ping(conn)

	server := &WebSocketServer{
		conn:         conn,
		responseChan: make(chan messages.HttpResponseMessage),
		config:       cfg,
	}

	server.messageWG.Add(1)
	go server.listenForMessages()

	return server, nil
}
