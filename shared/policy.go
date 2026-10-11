package shared

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/niradler/go-netbridge/config"
)

const maxBodyBytes = 1 << 20

var errNotAllowed = errors.New("request not allowed")

func secretOK(got, want string) bool {
	if len(want) < 16 || len(got) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func originAllowed(origin string, allowed []string) bool {
	if origin == "" {
		return true
	}
	for _, item := range allowed {
		if origin == item {
			return true
		}
	}
	return false
}

func RequestTarget(r *http.Request, cfg *config.Config) (*url.URL, error) {
	if cfg.PROXY_TYPE == "server" {
		base, err := url.Parse(cfg.SERVER_URL)
		if err != nil {
			return nil, errNotAllowed
		}
		base.Path = r.URL.Path
		base.RawQuery = r.URL.RawQuery
		base.Fragment = ""
		base.User = nil
		return base, nil
	}

	host := r.Host
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	if cfg.X_Forwarded_Host != "" {
		host = cfg.X_Forwarded_Host
	}
	if cfg.X_Forwarded_Proto != "" {
		proto = cfg.X_Forwarded_Proto
	}
	if cfg.TRUST_FORWARDED {
		if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
		}
		if forwardedProto := r.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
			proto = forwardedProto
		}
	}
	if host == "" || (proto != "http" && proto != "https") {
		return nil, errNotAllowed
	}
	return &url.URL{Scheme: proto, Host: host, Path: r.URL.Path, RawQuery: r.URL.RawQuery}, nil
}

func HostAllowed(target *url.URL, cfg *config.Config) error {
	if target == nil || target.User != nil || target.Host == "" {
		return errNotAllowed
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return errNotAllowed
	}
	host := strings.ToLower(target.Hostname())
	for _, allowed := range allowedHosts(cfg) {
		if strings.EqualFold(host, allowed) || strings.EqualFold(target.Host, allowed) {
			return nil
		}
	}
	return errNotAllowed
}

func allowedHosts(cfg *config.Config) []string {
	hosts := append([]string{}, cfg.WHITE_LIST...)
	if cfg.SERVER_URL != "" {
		if parsed, err := url.Parse(cfg.SERVER_URL); err == nil && parsed.Hostname() != "" {
			hosts = append(hosts, parsed.Hostname())
		}
	}
	return hosts
}

func validMethod(method string) bool {
	if method == "" || len(method) > 16 {
		return false
	}
	for _, char := range method {
		if char < 'A' || char > 'Z' {
			return false
		}
	}
	return true
}
