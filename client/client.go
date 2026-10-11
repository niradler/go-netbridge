package main

import (
	"log"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/shared"
)

func main() {
	cfg, err := config.LoadConfig(nil)
	if err != nil {
		log.Fatal(err)
	}
	shared.InitLogger(*cfg)
	client, err := shared.Dial(cfg)
	if err != nil {
		log.Fatal(err)
	}
	server := shared.NewHTTPServer(cfg, nil)
	shared.Attach(server, client)
	log.Fatal(server.Start())
}
