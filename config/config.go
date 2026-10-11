package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	X_Forwarded_Host     string
	X_Forwarded_Proto    string
	PORT                 string
	BIND_ADDR            string
	SSL_CERT_FILE        string
	SSL_KEY_FILE         string
	REQUEST_CA_FILE      string
	INSECURE_SKIP_VERIFY bool
	TRUST_FORWARDED      bool
	LOG_LEVEL            string
	LOG_JSON             bool
	LOG_FILE             string
	Type                 string
	SERVER_URL           string
	SOCKET_URL           string
	SECRET               string
	PROXY_TYPE           string
	WHITE_LIST           []string
	ALLOWED_ORIGINS      []string
}

func LoadConfig(userConfig *Config) (*Config, error) {
	_ = godotenv.Load()

	envConfig := Config{
		X_Forwarded_Host:     os.Getenv("X_FORWARDED_HOST"),
		X_Forwarded_Proto:    os.Getenv("X_FORWARDED_PROTO"),
		PORT:                 os.Getenv("PORT"),
		BIND_ADDR:            os.Getenv("BIND_ADDR"),
		SSL_CERT_FILE:        os.Getenv("SSL_CERT_FILE"),
		SSL_KEY_FILE:         os.Getenv("SSL_KEY_FILE"),
		REQUEST_CA_FILE:      os.Getenv("REQUEST_CA_FILE"),
		INSECURE_SKIP_VERIFY: os.Getenv("INSECURE_SKIP_VERIFY") == "true",
		TRUST_FORWARDED:      os.Getenv("TRUST_FORWARDED") == "true",
		LOG_LEVEL:            os.Getenv("LOG_LEVEL"),
		LOG_JSON:             os.Getenv("LOG_JSON") == "true",
		LOG_FILE:             os.Getenv("LOG_FILE"),
		Type:                 os.Getenv("TUNNEL_TYPE"),
		SERVER_URL:           os.Getenv("SERVER_URL"),
		SOCKET_URL:           os.Getenv("SOCKET_URL"),
		SECRET:               os.Getenv("SECRET"),
		PROXY_TYPE:           os.Getenv("PROXY_TYPE"),
		WHITE_LIST:           splitList(os.Getenv("WHITE_LIST")),
		ALLOWED_ORIGINS:      splitList(os.Getenv("ALLOWED_ORIGINS")),
	}

	config := envConfig
	if userConfig != nil {
		config.X_Forwarded_Host = mergeString(envConfig.X_Forwarded_Host, userConfig.X_Forwarded_Host)
		config.X_Forwarded_Proto = mergeString(envConfig.X_Forwarded_Proto, userConfig.X_Forwarded_Proto)
		config.PORT = mergeString(envConfig.PORT, userConfig.PORT)
		config.BIND_ADDR = mergeString(envConfig.BIND_ADDR, userConfig.BIND_ADDR)
		config.SSL_CERT_FILE = mergeString(envConfig.SSL_CERT_FILE, userConfig.SSL_CERT_FILE)
		config.SSL_KEY_FILE = mergeString(envConfig.SSL_KEY_FILE, userConfig.SSL_KEY_FILE)
		config.REQUEST_CA_FILE = mergeString(envConfig.REQUEST_CA_FILE, userConfig.REQUEST_CA_FILE)
		config.INSECURE_SKIP_VERIFY = userConfig.INSECURE_SKIP_VERIFY || envConfig.INSECURE_SKIP_VERIFY
		config.TRUST_FORWARDED = userConfig.TRUST_FORWARDED || envConfig.TRUST_FORWARDED
		config.LOG_LEVEL = mergeString(envConfig.LOG_LEVEL, userConfig.LOG_LEVEL)
		config.LOG_JSON = userConfig.LOG_JSON || envConfig.LOG_JSON
		config.LOG_FILE = mergeString(envConfig.LOG_FILE, userConfig.LOG_FILE)
		config.Type = mergeString(envConfig.Type, userConfig.Type)
		config.SERVER_URL = mergeString(envConfig.SERVER_URL, userConfig.SERVER_URL)
		config.SOCKET_URL = mergeString(envConfig.SOCKET_URL, userConfig.SOCKET_URL)
		config.SECRET = mergeString(envConfig.SECRET, userConfig.SECRET)
		config.PROXY_TYPE = mergeString(envConfig.PROXY_TYPE, userConfig.PROXY_TYPE)
		if len(userConfig.WHITE_LIST) > 0 {
			config.WHITE_LIST = userConfig.WHITE_LIST
		}
		if len(userConfig.ALLOWED_ORIGINS) > 0 {
			config.ALLOWED_ORIGINS = userConfig.ALLOWED_ORIGINS
		}
	}

	if config.PORT == "" {
		config.PORT = "8081"
	}
	if config.BIND_ADDR == "" {
		config.BIND_ADDR = "127.0.0.1"
	}
	if config.LOG_LEVEL == "" {
		config.LOG_LEVEL = "info"
	}
	if config.Type == "" {
		config.Type = "client"
	}
	if config.PROXY_TYPE == "" {
		config.PROXY_TYPE = "wss"
	}
	if err := validate(&config); err != nil {
		return nil, err
	}
	return &config, nil
}

func validate(config *Config) error {
	if len(config.SECRET) < 16 {
		return errors.New("SECRET must be at least 16 characters")
	}
	if config.Type != "client" && config.Type != "server" {
		return errors.New("TUNNEL_TYPE must be client or server")
	}
	switch config.PROXY_TYPE {
	case "wss", "server", "proxy":
	default:
		return errors.New("PROXY_TYPE must be wss, server, or proxy")
	}
	if net.ParseIP(config.BIND_ADDR) == nil && config.BIND_ADDR != "localhost" {
		return errors.New("BIND_ADDR must be an IP address or localhost")
	}
	if (config.SSL_CERT_FILE == "") != (config.SSL_KEY_FILE == "") {
		return errors.New("SSL_CERT_FILE and SSL_KEY_FILE must be set together")
	}
	if config.Type == "client" {
		parsed, err := url.Parse(config.SOCKET_URL)
		if err != nil || (parsed.Scheme != "ws" && parsed.Scheme != "wss") || parsed.Host == "" || parsed.User != nil {
			return errors.New("SOCKET_URL must be an absolute ws:// or wss:// URL")
		}
	}
	if config.PROXY_TYPE == "server" || config.SERVER_URL != "" {
		parsed, err := url.Parse(config.SERVER_URL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("SERVER_URL must be an absolute http(s) URL")
		}
	}
	return nil
}

func mergeString(envValue, userValue string) string {
	if userValue != "" {
		return userValue
	}
	return envValue
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var result []string
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}
