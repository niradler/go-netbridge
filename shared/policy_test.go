package shared

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/niradler/go-netbridge/config"
)

func TestHostAllowedRejectsPrefixLookalikes(t *testing.T) {
	cfg := &config.Config{WHITE_LIST: []string{"example.com"}}
	if HostAllowed(mustURL(t, "https://example.com/path"), cfg) != nil {
		t.Fatal("expected example.com to be allowed")
	}
	if HostAllowed(mustURL(t, "https://example.com.evil.test/path"), cfg) == nil {
		t.Fatal("prefix lookalike was allowed")
	}
	if HostAllowed(mustURL(t, "http://127.0.0.1/latest/meta-data"), cfg) == nil {
		t.Fatal("metadata address was allowed")
	}
}

func TestRequestTargetIgnoresClientForwardedHost(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://proxy.local/app?x=1", nil)
	request.Host = "app.internal"
	request.Header.Set("X-Forwarded-Host", "evil.test")
	request.Header.Set("X-Forwarded-Proto", "https")
	target, err := RequestTarget(request, &config.Config{PROXY_TYPE: "wss"})
	if err != nil {
		t.Fatal(err)
	}
	if target.Host != "app.internal" || target.Scheme != "http" || target.RawQuery != "x=1" {
		t.Fatalf("target = %s", target.String())
	}
}

func TestSecretCompare(t *testing.T) {
	secret := "0123456789abcdef"
	if !secretOK(secret, secret) {
		t.Fatal("expected matching secret")
	}
	if secretOK(secret+"x", secret) || secretOK("short", secret) || secretOK("", secret) {
		t.Fatal("mismatched secret was accepted")
	}
}
