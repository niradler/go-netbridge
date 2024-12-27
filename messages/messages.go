package messages

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

var MessageType = struct {
	Response string
	Ping     string
	Request  string
	Error    string
}{
	Ping:     "ping",
	Request:  "request",
	Response: "response",
	Error:    "error",
}

type HttpRequestMessage struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

type HttpResponseMessage struct {
	StatusCode int                 `json:"statusCode"`
	Headers    map[string][]string `json:"headers"`
	Body       string              `json:"body"`
}

type PingMessage struct {
	Body string `json:"body"`
}

type ErrorMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Message struct {
	Type     string      `json:"type"`
	Params   interface{} `json:"params"`
	Chunk    int         `json:"chunk"`
	Total    int         `json:"total"`
	Response bool        `json:"response,omitempty"`
	ID       string      `json:"id"`
}

func parseMessage[T any](msg Message, target *T) (Message, error) {
	params, err := json.Marshal(msg.Params)
	if err != nil {
		return msg, err
	}
	err = json.Unmarshal(params, target)
	if err != nil {
		return msg, err
	}
	msg.Params = target

	return msg, nil
}

func ReadAndParseMessage(conn *websocket.Conn) (Message, error) {
	var msg Message
	err := conn.ReadJSON(&msg)
	if err != nil {
		return msg, err
	}

	switch msg.Type {
	case MessageType.Ping:
		var pingMsg PingMessage
		return parseMessage(msg, &pingMsg)
	case MessageType.Request:
		var httpMsg HttpRequestMessage
		return parseMessage(msg, &httpMsg)
	case MessageType.Response:
		var resMsg HttpResponseMessage
		return parseMessage(msg, &resMsg)
	case MessageType.Error:
		var errorMsg ErrorMessage
		return parseMessage(msg, &errorMsg)
	default:
		return msg, errors.New("unknown message type")
	}
}

func ReadChunks(conn *websocket.Conn, msg Message) ([]Message, error) {
	var messages []Message
	messages = append(messages, msg)
	for i := 0; i < msg.Total; i++ {
		err := conn.ReadJSON(&msg)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
		if msg.Chunk == msg.Total {
			break
		}
	}
	return messages, nil
}

func CreateId() string {
	id := fmt.Sprintf("%x", time.Now().UnixNano())
	return "msg_" + id[:16]
}
