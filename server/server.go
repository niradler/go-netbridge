package main

import (
	"log"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/shared"
)

func main() {
	cfg, err := config.LoadConfig(&config.Config{
		PORT: "8080",
		Type: "server",
	})
	if err != nil {
		log.Fatal(err)
	}

	shared.InitLogger(*cfg)

	httpServer := shared.NewHTTPServer(cfg, nil)

	shared.NewWebSocketServer(httpServer)

	log.Fatal(httpServer.Start())
}
