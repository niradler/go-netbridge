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

type HttpRequestMessageParams struct {
	Method  string              `json:"method"`
	URL     string              `json:"url"`
	Headers map[string][]string `json:"headers"`
	Body    string              `json:"body"`
}

type HttpResponseMessageParams struct {
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

type HttpRequestMessage struct {
	Message
	Params HttpRequestMessageParams `json:"params"`
}

type HttpResponseMessage struct {
	Message
	Params HttpResponseMessageParams `json:"params"`
}

type Message struct {
	Type     string      `json:"type"`
	Params   interface{} `json:"params"`
	Chunk    int         `json:"chunk"`
	Total    int         `json:"total"`
	Response bool        `json:"response,omitempty"`
	ID       string      `json:"id"`
}

func ParseMessageParams[T any](params interface{}, target *T) (T, error) {
	var empty T
	data, err := json.Marshal(params)
	if err != nil {

		return empty, err
	}
	err = json.Unmarshal(data, target)
	if err != nil {
		return empty, err
	}

	return *target, nil
}

func ReadAndParseMessage(conn *websocket.Conn) (Message, error) {
	var msg Message
	err := conn.ReadJSON(&msg)
	if err != nil {
		return msg, err
	}

	switch msg.Type {
	case MessageType.Ping:
		return msg, nil
	case MessageType.Request:
		return msg, nil
	case MessageType.Response:
		return msg, nil
	case MessageType.Error:
		return msg, nil
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
