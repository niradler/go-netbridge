package main

import (
	"log"

	"github.com/niradler/go-netbridge/config"
	"github.com/niradler/go-netbridge/tools"
)

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	cfg.PORT = "8080"
	cfg.Type = "server"
	server := tools.NewServer(cfg)
	server.Start()
}
