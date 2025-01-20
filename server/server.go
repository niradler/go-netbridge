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
	shared.InitLogger(*cfg)
	cfg.PORT = "8080"
	cfg.Type = "server"
	httpServer := shared.NewHTTPServer(cfg, nil)

	shared.NewWebSocketServer(httpServer)

	log.Fatal(httpServer.Start())
}
