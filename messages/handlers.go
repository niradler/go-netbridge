package messages

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/tunnel"
	"github.com/valyala/fasthttp"
)

func HttpRequest(requestParams *HttpRequestMessage, config *config.Config) (*Message, error) {
	log.Printf("HttpRequestMessage: %v  %v bodylen=%v", requestParams.Method, requestParams.URL, len(requestParams.Body))

	req := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(req)
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(requestParams.URL)
	req.Header.SetMethod(requestParams.Method)
	for key, value := range requestParams.Headers {
		for _, v := range value {
			req.Header.Add(key, v)
		}
	}
	req.SetBodyString(requestParams.Body)

	client := &fasthttp.Client{
		ReadBufferSize:      16 * 1024,
		WriteBufferSize:     16 * 1024,
		MaxConnDuration:     30 * time.Minute,
		MaxIdleConnDuration: 60 * time.Second,
	}

	if config.REQUEST_CA_FILE != "" {
		caCert, err := os.ReadFile(config.REQUEST_CA_FILE)
		if err != nil {
			return nil, err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		client.TLSConfig = &tls.Config{
			RootCAs: caCertPool,
		}
	}

	err := client.DoRedirects(req, resp, 10)
	if err != nil {
		return nil, err
	}

	var body string
	if resp.Header.Peek("Content-Encoding") != nil && string(resp.Header.Peek("Content-Encoding")) == "gzip" {
		bodyBytes, err := resp.BodyGunzip()
		if err != nil {
			return nil, err
		}
		body = string(bodyBytes)
	} else {
		body = string(resp.Body())
	}

	headers := make(map[string][]string)
	resp.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = append(headers[string(key)], string(value))
	})

	responseMsg := Message{
		Type:     MessageType.Response,
		Params:   HttpResponseMessage{StatusCode: resp.StatusCode(), Headers: headers, Body: body},
		Response: true,
		ID:       CreateId(),
	}

	log.Printf("HttpResponseMessage, StatusCode=%v Method=%v URL=%v bodylen=%v", resp.StatusCode(), requestParams.Method, requestParams.URL, len(body))

	return &responseMsg, nil

}

func MessageHandler(conn *websocket.Conn, msg Message, config config.Config) error {
	if conn == nil {
		fmt.Println("Connection is not open")
		return errors.New("connection is not open")
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
		responseParams, ok := msg.Params.(*HttpRequestMessage)
		if !ok {
			return errors.New("failed to parse request message")
		}
		responseMsg, err := HttpRequest(responseParams, &config)
		if err != nil {
			return err
		}
		err = tunnel.WriteJSON(conn, responseMsg)
		if err != nil {
			return err
		}
	default:
		return errors.New("unknown message type")
	}
	return nil
}
