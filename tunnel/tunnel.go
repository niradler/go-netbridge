package tunnel

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/socketflow"
)

const maxMessageSize = 4 << 20

func Create(w http.ResponseWriter, r *http.Request, cfg config.Config) (*socketflow.WebSocketClient, error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
		CheckOrigin: func(request *http.Request) bool {
			origin := request.Header.Get("Origin")
			if origin == "" {
				return true
			}
			for _, allowed := range cfg.ALLOWED_ORIGINS {
				if origin == allowed {
					return true
				}
			}
			return false
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(maxMessageSize)
	return socketflow.NewWebSocketClient(conn, socketflow.Config{
		ChunkSize:        1 << 20,
		ReassembleChunks: true,
	}), nil
}

func Connect(cfg *config.Config) (*socketflow.WebSocketClient, error) {
	parsed, err := url.Parse(cfg.SOCKET_URL)
	if err != nil {
		return nil, err
	}
	headers := http.Header{}
	headers.Set("X-Auth-Secret", cfg.SECRET)
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig: &tls.Config{
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: cfg.INSECURE_SKIP_VERIFY,
		},
	}
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		conn, _, err := dialer.Dial(parsed.String(), headers)
		if err == nil {
			conn.SetReadLimit(maxMessageSize)
			return socketflow.NewWebSocketClient(conn, socketflow.Config{
				ChunkSize:        1 << 20,
				ReassembleChunks: true,
			}), nil
		}
		lastErr = err
		log.Printf("websocket connect attempt %d failed", attempt)
		time.Sleep(time.Second)
	}
	return nil, lastErr
}
