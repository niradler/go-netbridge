package messages

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/websocket"
	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/tunnel"
	"github.com/valyala/fasthttp"
)

const MaxChunkSize = 10 * 1024 * 1024 // 10 MB

func HttpRequestHandler(id string, conn *websocket.Conn, requestParams *HttpRequestMessage, config *config.Config) error {
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
		ReadBufferSize:      MaxChunkSize,
		WriteBufferSize:     MaxChunkSize,
		MaxConnDuration:     5 * time.Minute,
		MaxIdleConnDuration: 60 * time.Second,
	}

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

	err := client.DoRedirects(req, resp, 10)
	if err != nil {
		return err
	}

	headers := make(map[string][]string)
	resp.Header.VisitAll(func(key, value []byte) {
		headers[string(key)] = append(headers[string(key)], string(value))
	})

	totalSize := resp.Header.ContentLength()
	chunkCounter := 0

	for {
		chunk := make([]byte, MaxChunkSize)
		n, err := resp.BodyStream().Read(chunk)
		if err != nil {
			log.Printf("Error reading response body: %v", err)
			if err == http.ErrBodyReadAfterClose {
				break
			}

			return err
		}

		chunkCounter++
		responseMsg := Message{
			Type:     MessageType.Response,
			Params:   HttpResponseMessage{StatusCode: resp.StatusCode(), Headers: headers, Body: string(chunk)},
			Response: true,
			ID:       id,
			Total:    int(totalSize),
			Chunk:    chunkCounter,
		}

		log.Printf("HttpResponseMessage, StatusCode=%v Method=%v URL=%v, Chunk=%v", resp.StatusCode(), requestParams.Method, requestParams.URL, responseMsg.Chunk)
		err = conn.WriteJSON(responseMsg)
		if err != nil {
			return err
		}

		if n < MaxChunkSize {
			break
		}
	}

	return nil

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
		err := HttpRequestHandler(msg.ID, conn, responseParams, &config)
		if err != nil {
			log.Printf("Error handling request: %v", err)
			err = conn.WriteJSON(Message{
				Type:     MessageType.Response,
				Params:   HttpResponseMessage{StatusCode: 500, Headers: map[string][]string{"Content-Type": {"application/json"}}, Body: `{"error": "Request failed"}`},
				Response: true,
				ID:       msg.ID,
				Total:    1,
				Chunk:    1,
			})
			if err != nil {
				return err
			}
		}

	default:
		return errors.New("unknown message type")
	}
	return nil
}
