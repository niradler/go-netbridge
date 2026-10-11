package main

import (
	"log"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/shared"
)

func main() {
	cfg, err := config.LoadConfig(&config.Config{Type: "server"})
	if err != nil {
		log.Fatal(err)
	}
	shared.InitLogger(*cfg)
	server := shared.NewHTTPServer(cfg, nil)
	shared.MountWebSocket(server)
	log.Fatal(server.Start())
}
