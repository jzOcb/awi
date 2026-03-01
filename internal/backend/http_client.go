package backend

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	xproxy "golang.org/x/net/proxy"
)

type contextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func newHTTPClientWithProxy(timeout time.Duration, proxyRaw string) (*http.Client, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	proxyRaw = strings.TrimSpace(proxyRaw)
	if proxyRaw != "" {
		proxyURL, err := url.Parse(proxyRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}

		switch strings.ToLower(proxyURL.Scheme) {
		case "http", "https":
			transport.Proxy = http.ProxyURL(proxyURL)
		case "socks5", "socks5h":
			dialer, err := xproxy.FromURL(proxyURL, xproxy.Direct)
			if err != nil {
				return nil, fmt.Errorf("invalid socks5 proxy: %w", err)
			}
			transport.Proxy = nil
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				if cd, ok := dialer.(contextDialer); ok {
					return cd.DialContext(ctx, network, address)
				}
				return dialer.Dial(network, address)
			}
		default:
			return nil, fmt.Errorf("unsupported proxy scheme: %s", proxyURL.Scheme)
		}
	}

	return &http.Client{Timeout: timeout, Transport: transport}, nil
}
