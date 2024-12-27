package main

import (
	"log"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/shared"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	cfg.PORT = "8081"
	cfg.Type = "client"
	wss, err := shared.NewWebSocketConnection(cfg)
	if err != nil {
		log.Fatalf("Error creating WebSocket server: %v", err)
	}
	defer wss.Close()

	httpServer := shared.NewHTTPServer(cfg, wss)
	log.Fatal(httpServer.Start())
}
