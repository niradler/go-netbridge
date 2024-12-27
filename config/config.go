package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	X_Forwarded_Host     string
	X_Forwarded_Proto    string
	PORT                 string
	SSL_CERT_FILE        string
	SSL_KEY_FILE         string
	REQUEST_CA_FILE      string
	INSECURE_SKIP_VERIFY bool
	LOG_LEVEL            string
	LOG_JSON             bool
	LOG_FILE             string
	Type                 string
	SERVER_URL           string
	SECRET               string
}

func LoadConfig() (*Config, error) {
	godotenv.Load()

	config := &Config{
		X_Forwarded_Host:     os.Getenv("X_FORWARDED_HOST"),
		X_Forwarded_Proto:    os.Getenv("X_FORWARDED_PROTO"),
		PORT:                 os.Getenv("PORT"),
		SSL_CERT_FILE:        os.Getenv("SSL_CERT_FILE"),
		SSL_KEY_FILE:         os.Getenv("SSL_KEY_FILE"),
		REQUEST_CA_FILE:      os.Getenv("REQUEST_CA_FILE"),
		INSECURE_SKIP_VERIFY: os.Getenv("INSECURE_SKIP_VERIFY") == "true",
		LOG_LEVEL:            os.Getenv("LOG_LEVEL"),
		LOG_JSON:             os.Getenv("LOG_JSON") == "true",
		LOG_FILE:             os.Getenv("LOG_FILE"),
		Type:                 os.Getenv("TUNNEL_TYPE"),
		SERVER_URL:           os.Getenv("SERVER_URL"),
		SECRET:               os.Getenv("SECRET"),
	}

	if config.PORT == "" {
		config.PORT = "8081"
	}

	if config.LOG_LEVEL == "" {
		config.LOG_LEVEL = "info"
	}

	if config.Type == "" {
		config.Type = "client"
	}

	if config.SERVER_URL != "" && config.Type == "server" {
		panic("SERVER_URL is only for client")
	}

	if config.SERVER_URL == "" && config.Type == "client" {
		panic("SERVER_URL is mandatory for client")
	}

	return config, nil
}
