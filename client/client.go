package main

import (
	"log"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/shared"
)

func main() {
	cfg, err := config.LoadConfig(&config.Config{
		PORT:       "8081",
		Type:       "client",
		SOCKET_URL: "ws://localhost:8080/_ws",
		SERVER_URL: "http://localhost:8080",
	})
	if err != nil {
		log.Fatal(err)
	}

	shared.InitLogger(*cfg)

	wss, err := shared.NewWebSocketConnection(cfg)
	if err != nil {
		log.Fatalf("Error creating WebSocket server: %v", err)
	}

	defer wss.Close()

	httpServer := shared.NewHTTPServer(cfg, wss.Client)
	log.Fatal(httpServer.Start())
}
