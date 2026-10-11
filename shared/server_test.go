package shared

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/niradler/go-netbridge/config"
)

func TestMain(m *testing.M) {
	InitLogger(config.Config{LOG_LEVEL: "error"})
	os.Exit(m.Run())
}

func TestServerModeForwardsToConfiguredUpstream(t *testing.T) {
	var gotPath string
	var gotSecret string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		gotSecret = r.Header.Get("X-Auth-Secret")
		http.SetCookie(w, &http.Cookie{Name: "a", Value: "1"})
		http.SetCookie(w, &http.Cookie{Name: "b", Value: "2"})
		_, _ = w.Write([]byte("upstream-body"))
	}))
	defer upstream.Close()

	server := NewHTTPServer(testConfig(upstream.URL), nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/items?q=1", nil)
	request.Header.Set("X-Auth-Secret", "0123456789abcdef")
	request.Header.Set("X-Forwarded-Host", "evil.test")
	server.router.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != "upstream-body" {
		t.Fatalf("status %d body %q", response.Code, response.Body.String())
	}
	if gotPath != "/items?q=1" {
		t.Fatalf("upstream path = %s", gotPath)
	}
	if gotSecret != "" {
		t.Fatal("tunnel secret was forwarded upstream")
	}
	if len(response.Result().Cookies()) != 2 {
		t.Fatalf("cookies = %d", len(response.Result().Cookies()))
	}
}

func TestUnlistedHostIsRejected(t *testing.T) {
	cfg := testConfig("http://app.internal")
	cfg.PROXY_TYPE = "wss"
	cfg.SERVER_URL = ""
	cfg.WHITE_LIST = []string{"app.internal"}
	server := NewHTTPServer(cfg, nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "http://evil.test/secret", nil)
	request.Host = "evil.test"
	server.router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status %d", response.Code)
	}
}

func TestWebSocketRequiresSecret(t *testing.T) {
	cfg := testConfig("http://app.internal")
	cfg.PROXY_TYPE = "wss"
	mounted := NewHTTPServer(cfg, nil)
	MountWebSocket(mounted)
	live := httptest.NewServer(mounted.router)
	defer live.Close()

	endpoint := "ws" + strings.TrimPrefix(live.URL, "http") + "/_ws"
	_, response, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err == nil {
		t.Fatal("expected the upgrade to be rejected")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %v", response)
	}

	header := http.Header{}
	header.Set("X-Auth-Secret", cfg.SECRET)
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, header)
	if err != nil {
		t.Fatal(err)
	}
	_ = conn.Close()
}

func TestUpstreamBodySurvivesClientRelease(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "payload-remains")
	}))
	defer upstream.Close()
	parsed, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(upstream.URL)
	cfg.WHITE_LIST = []string{parsed.Hostname()}
	response, err := HttpRequest(&HttpRequestMessage{
		RequestID: "1",
		Method:    http.MethodGet,
		URL:       upstream.URL + "/data",
	}, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if string(response.Body) != "payload-remains" || response.RequestID != "1" {
		t.Fatalf("response = %+v", response)
	}
}

func testConfig(serverURL string) *config.Config {
	return &config.Config{
		SECRET:     "0123456789abcdef",
		Type:       "server",
		PROXY_TYPE: "server",
		SERVER_URL: serverURL,
		PORT:       "8080",
		BIND_ADDR:  "127.0.0.1",
		LOG_LEVEL:  "error",
		WHITE_LIST: nil,
	}
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}
