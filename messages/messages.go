package messages

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

var MessageType = struct {
	Response string
	Ping     string
	Http     string
	Error    string
}{
	Response: "response",
	Ping:     "ping",
	Http:     "http",
	Error:    "error",
}

type HttpMessage struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
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
	Response interface{} `json:"response,omitempty"`
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
	case "response":
		var resMsg Message
		return parseMessage(msg, &resMsg)
	case "ping":
		var pingMsg PingMessage
		return parseMessage(msg, &pingMsg)
	case "http":
		var httpMsg HttpMessage
		return parseMessage(msg, &httpMsg)
	case "error":
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

func MessageHandler(conn *websocket.Conn, msg Message) error {
	switch msg.Type {
	case MessageType.Ping:
		pingMsg := msg.Params.(*PingMessage)
		if pingMsg.Body == "ping" {
			msg.Response = "pong"
			err := conn.WriteJSON(Message{
				Type:     MessageType.Response,
				Params:   pingMsg,
				Response: msg.Response,
				ID:       msg.ID,
			})
			if err != nil {
				return err
			}
		}
	case MessageType.Http:
		httpMsg := msg.Params.(*HttpMessage)
		client := &http.Client{}
		req, err := http.NewRequest(httpMsg.Method, httpMsg.URL, strings.NewReader(httpMsg.Body))
		if err != nil {
			return err
		}
		for key, value := range httpMsg.Headers {
			req.Header.Add(key, value)
		}
		resp, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		msg.Response = string(body)
		err = conn.WriteJSON(Message{
			Type:     MessageType.Response,
			Response: msg.Response,
			ID:       msg.ID,
		})
		if err != nil {
			return err
		}
	default:
		return errors.New("unknown message type")
	}
	return nil
}

func CreateId() string {
	return "msg_id_" + fmt.Sprintf("%d", time.Now().UnixNano())
}
