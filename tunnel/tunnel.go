package tunnel

import (
	"log"
	"net/http"
	"net/url"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func Create(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Error upgrading connection:", err)
		return conn, err
	}

	return conn, nil
}

func Connect(url url.URL) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
		return conn, err
	}

	return conn, nil
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

func SendJSON(conn *websocket.Conn, v interface{}) error {
	err := conn.WriteJSON(v)
	if err != nil {
		log.Println("Error sending JSON:", err)
	}
	return err
}

func ReceiveJSON(conn *websocket.Conn, v interface{}) error {
	err := conn.ReadJSON(v)
	if err != nil {
		log.Println("Error receiving JSON:", err)
	}
	return err
}
