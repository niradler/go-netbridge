package messages

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/tunnel"
	"github.com/valyala/fasthttp"
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
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

type HttpResponseMessage struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
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

func MessageHandler(conn *websocket.Conn, msg Message, config config.Config) error {
	if conn == nil {
		fmt.Println("Connection is not open")
		return errors.New("connection is not open")
	} else {
		fmt.Println("Connection is open")
	}
	switch msg.Type {
	case MessageType.Ping:
		pingMsg := msg.Params.(*PingMessage)
		if pingMsg.Body == "ping" {
			err := tunnel.WriteJSON(conn, Message{
				Type: MessageType.Ping,
				Params: PingMessage{
					Body: "pong",
				},
				Response: true,
				ID:       msg.ID,
			})
			if err != nil {
				return err
			}
		}
	case MessageType.Request:
		httpMsg := msg.Params.(*HttpRequestMessage)
		req := fasthttp.AcquireRequest()
		defer fasthttp.ReleaseRequest(req)
		resp := fasthttp.AcquireResponse()
		defer fasthttp.ReleaseResponse(resp)

		req.SetRequestURI(httpMsg.URL)
		req.Header.SetMethod(httpMsg.Method)
		for key, value := range httpMsg.Headers {
			req.Header.Set(key, value)
		}
		req.SetBodyString(httpMsg.Body)

		client := &fasthttp.Client{}

		if config.REQUEST_CA_FILE != "" {
			caCert, err := os.ReadFile(config.REQUEST_CA_FILE)
			if err != nil {
				return err
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCert)
			client.TLSConfig = &tls.Config{
				RootCAs: caCertPool,
			}
		}

		err := client.Do(req, resp)
		if err != nil {
			return err
		}

		body := string(resp.Body())
		headers := make(map[string]string)
		resp.Header.VisitAll(func(key, value []byte) {
			headers[string(key)] = string(value)
		})

		msg.Response = true
		err = tunnel.WriteJSON(conn, Message{
			Type:     MessageType.Response,
			Params:   HttpResponseMessage{StatusCode: resp.StatusCode(), Headers: headers, Body: body},
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
	id := fmt.Sprintf("%x", time.Now().UnixNano())
	return "msg_" + id[:16]
}
