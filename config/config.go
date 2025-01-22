package config

import (
	"log"
	"os"

	"strings"

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
	SOCKET_URL           string
	SECRET               string
	PROXY_TYPE           string
	WHITE_LIST           []string
}

func filterEmpty(slice []string) []string {
	var result []string
	for _, str := range slice {
		if str != "" {
			result = append(result, str)
		}
	}
	return result
}

func LoadConfig(userConfig *Config) (*Config, error) {
	godotenv.Load()

	envConfig := Config{
		X_Forwarded_Host:     os.Getenv("X_FORWARDED_HOST"),
		X_Forwarded_Proto:    os.Getenv("X_FORWARDED_PROTO"),
		PORT:                 os.Getenv("PORT"),
		SSL_CERT_FILE:        os.Getenv("SSL_CERT_FILE"),
		WHITE_LIST:           filterEmpty(strings.Split(os.Getenv("WHITE_LIST"), ",")),
		REQUEST_CA_FILE:      os.Getenv("REQUEST_CA_FILE"),
		INSECURE_SKIP_VERIFY: os.Getenv("INSECURE_SKIP_VERIFY") == "true",
		LOG_LEVEL:            os.Getenv("LOG_LEVEL"),
		LOG_JSON:             os.Getenv("LOG_JSON") == "true",
		LOG_FILE:             os.Getenv("LOG_FILE"),
		Type:                 os.Getenv("TUNNEL_TYPE"),
		SERVER_URL:           os.Getenv("SERVER_URL"),
		SOCKET_URL:           os.Getenv("SOCKET_URL"),
		SECRET:               os.Getenv("SECRET"),
	}

	mergeConfig := func(envValue, userValue string) string {
		if userValue != "" {
			return userValue
		}
		return envValue
	}

	config := &Config{
		X_Forwarded_Host:     envConfig.X_Forwarded_Host,
		X_Forwarded_Proto:    envConfig.X_Forwarded_Proto,
		PORT:                 envConfig.PORT,
		SSL_CERT_FILE:        envConfig.SSL_CERT_FILE,
		SSL_KEY_FILE:         envConfig.SSL_KEY_FILE,
		REQUEST_CA_FILE:      envConfig.REQUEST_CA_FILE,
		INSECURE_SKIP_VERIFY: envConfig.INSECURE_SKIP_VERIFY,
		LOG_LEVEL:            envConfig.LOG_LEVEL,
		LOG_JSON:             envConfig.LOG_JSON,
		LOG_FILE:             envConfig.LOG_FILE,
		Type:                 envConfig.Type,
		SERVER_URL:           envConfig.SERVER_URL,
		SOCKET_URL:           envConfig.SOCKET_URL,
		SECRET:               envConfig.SECRET,
		PROXY_TYPE:           envConfig.PROXY_TYPE,
		WHITE_LIST:           envConfig.WHITE_LIST,
	}

	if userConfig != nil {
		config.X_Forwarded_Host = mergeConfig(envConfig.X_Forwarded_Host, userConfig.X_Forwarded_Host)
		config.X_Forwarded_Proto = mergeConfig(envConfig.X_Forwarded_Proto, userConfig.X_Forwarded_Proto)
		config.PORT = mergeConfig(envConfig.PORT, userConfig.PORT)
		config.SSL_CERT_FILE = mergeConfig(envConfig.SSL_CERT_FILE, userConfig.SSL_CERT_FILE)
		config.SSL_KEY_FILE = mergeConfig(envConfig.SSL_KEY_FILE, userConfig.SSL_KEY_FILE)
		config.REQUEST_CA_FILE = mergeConfig(envConfig.REQUEST_CA_FILE, userConfig.REQUEST_CA_FILE)
		config.INSECURE_SKIP_VERIFY = userConfig.INSECURE_SKIP_VERIFY || envConfig.INSECURE_SKIP_VERIFY
		config.LOG_LEVEL = mergeConfig(envConfig.LOG_LEVEL, userConfig.LOG_LEVEL)
		config.LOG_JSON = userConfig.LOG_JSON || envConfig.LOG_JSON
		config.LOG_FILE = mergeConfig(envConfig.LOG_FILE, userConfig.LOG_FILE)
		config.Type = mergeConfig(envConfig.Type, userConfig.Type)
		config.SERVER_URL = mergeConfig(envConfig.SERVER_URL, userConfig.SERVER_URL)
		config.SOCKET_URL = mergeConfig(envConfig.SOCKET_URL, userConfig.SOCKET_URL)
		config.SECRET = mergeConfig(envConfig.SECRET, userConfig.SECRET)
		config.PROXY_TYPE = mergeConfig(envConfig.PROXY_TYPE, userConfig.PROXY_TYPE)
		if len(userConfig.WHITE_LIST) > 0 {
			config.WHITE_LIST = userConfig.WHITE_LIST
		} else {
			config.WHITE_LIST = envConfig.WHITE_LIST
		}
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

	if config.PROXY_TYPE == "" {
		config.PROXY_TYPE = "wss"
	}

	log.Println("Config loaded", config)

	if config.SOCKET_URL == "" && config.Type == "client" {
		panic("SOCKET_URL is mandatory for client")
	}

	return config, nil
}
