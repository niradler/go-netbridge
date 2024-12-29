package tunnel

import (
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/go-netbridge/config"
)

const maxMessageSize = 6 * 1024 * 1024 // 6 MB in bytes

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  maxMessageSize,
	WriteBufferSize: maxMessageSize,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Create(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return conn, err
	}

	return conn, nil
}

func Connect(url url.URL, config config.Config) (*websocket.Conn, error) {
	headers := http.Header{}
	if config.SECRET != "" && config.Type == "client" {
		headers.Add("Authorization", config.SECRET)
	}

	var conn *websocket.Conn
	var err error
	for i := 0; i < 5; i++ {
		conn, _, err = websocket.DefaultDialer.Dial(url.String(), headers)
		if err == nil {
			return conn, nil
		}
		log.Printf("dial attempt %d failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	log.Fatal("dial failed after 5 attempts:", err)
	return conn, err
}

func Send(conn *websocket.Conn, msg []byte) error {
	err := conn.WriteMessage(websocket.TextMessage, msg)
	if err != nil {
		log.Println("Error sending message:", err)
	}
	return err
}

func Receive(conn *websocket.Conn) ([]byte, error) {
	_, msg, err := conn.ReadMessage()
	if err != nil {
		log.Println("Error reading message:", err)
	}
	return msg, err
}

func WriteJSON(conn *websocket.Conn, v interface{}) error {
	err := conn.WriteJSON(v)
	if err != nil {
		log.Println("Error sending JSON:", err)
	}
	return err
}

func ReadJSON(conn *websocket.Conn, v interface{}) error {
	err := conn.ReadJSON(v)
	if err != nil {
		log.Println("Error receiving JSON:", err)
	}
	return err
}
