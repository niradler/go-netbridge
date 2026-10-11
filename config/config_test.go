package config

import "testing"

func TestLoadConfigRequiresSecretAndReadsTLSKey(t *testing.T) {
	t.Setenv("SECRET", "short")
	t.Setenv("TUNNEL_TYPE", "server")
	if _, err := LoadConfig(nil); err == nil {
		t.Fatal("expected a short secret to be rejected")
	}

	t.Setenv("SECRET", "0123456789abcdef")
	t.Setenv("SSL_CERT_FILE", "/tmp/cert.pem")
	t.Setenv("SSL_KEY_FILE", "/tmp/key.pem")
	t.Setenv("PROXY_TYPE", "server")
	t.Setenv("SERVER_URL", "http://app.internal:8080")
	t.Setenv("BIND_ADDR", "")
	cfg, err := LoadConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SSL_KEY_FILE != "/tmp/key.pem" {
		t.Fatalf("ssl key = %q", cfg.SSL_KEY_FILE)
	}
	if cfg.BIND_ADDR != "127.0.0.1" {
		t.Fatalf("bind = %s", cfg.BIND_ADDR)
	}
	if cfg.PROXY_TYPE != "server" {
		t.Fatalf("proxy type = %s", cfg.PROXY_TYPE)
	}
}
