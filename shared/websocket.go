package shared

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/tunnel"
	"github.com/niradler/socketflow"
	"go.uber.org/zap"
)

func Dial(cfg *config.Config) (*socketflow.WebSocketClient, error) {
	return tunnel.Connect(cfg)
}

func MountWebSocket(server *HTTPServer) {
	server.router.Get("/_ws", server.serveWebSocket)
}

func Attach(server *HTTPServer, client *socketflow.WebSocketClient) {
	ctx, cancel := context.WithCancel(context.Background())
	responses := client.Subscribe("response")
	requests := client.Subscribe("request")
	status := client.SubscribeToStatus()
	server.setClient(client, cancel)
	go client.ReceiveMessages()
	go readResponses(ctx, responses, server.pending)
	go readRequests(ctx, requests, client, server.config)
	go func() {
		for item := range status {
			if item.Type == "error" || item.Type == "close" {
				cancel()
				return
			}
		}
	}()
}

func (hs *HTTPServer) serveWebSocket(w http.ResponseWriter, r *http.Request) {
	if !secretOK(r.Header.Get("X-Auth-Secret"), hs.config.SECRET) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	if !originAllowed(r.Header.Get("Origin"), hs.config.ALLOWED_ORIGINS) {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}
	client, err := tunnel.Create(w, r, *hs.config)
	if err != nil {
		GetLogger().Error("websocket upgrade failed")
		return
	}
	GetLogger().Info("websocket connection established")
	Attach(hs, client)
}

func readResponses(ctx context.Context, messages <-chan *socketflow.Message, pending *Pending) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok || message == nil {
				return
			}
			var response HttpResponseMessage
			if err := json.Unmarshal(message.Payload, &response); err != nil {
				continue
			}
			pending.Deliver(response)
		}
	}
}

func readRequests(ctx context.Context, messages <-chan *socketflow.Message, client *socketflow.WebSocketClient, cfg *config.Config) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-messages:
			if !ok || message == nil {
				return
			}
			var request HttpRequestMessage
			if err := json.Unmarshal(message.Payload, &request); err != nil {
				_ = SendResponseMessage(HttpResponseMessage{
					StatusCode: http.StatusBadRequest,
					Headers:    make(http.Header),
					Body:       []byte("Bad Request"),
				}, client)
				continue
			}
			if err := HttpRequestResponse(&request, cfg, client); err != nil {
				GetLogger().Error("failed to answer tunnel request", zap.Error(err))
			}
		}
	}
}
