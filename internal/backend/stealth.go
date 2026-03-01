package backend

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"github.com/go-shiori/go-readability"
)

type StealthBackend struct {
	timeout time.Duration
}

func NewStealthBackend(timeout time.Duration) *StealthBackend {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &StealthBackend{timeout: timeout}
}

func (b *StealthBackend) Name() string    { return "stealth" }
func (b *StealthBackend) Priority() int   { return 2 }
func (b *StealthBackend) Available() bool { return true }

func (b *StealthBackend) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
	if req == nil || strings.TrimSpace(req.URL) == "" {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, errors.New("missing URL"))
	}

	timeout := b.timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	timeoutMs := int(timeout / time.Millisecond)
	if timeoutMs <= 0 {
		timeoutMs = int((30 * time.Second) / time.Millisecond)
	}

	opts := []tls_client.HttpClientOption{
		tls_client.WithTimeoutMilliseconds(timeoutMs),
		tls_client.WithClientProfile(profiles.Chrome_120),
		tls_client.WithRandomTLSExtensionOrder(),
	}

	if proxyURL := strings.TrimSpace(req.Proxy); proxyURL != "" {
		opts = append(opts, tls_client.WithProxyUrl(proxyURL))
	}

	client, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(), opts...)
	if err != nil {
		return nil, NewBackendError(ErrUpstream, b.Name(), "read", true, err)
	}
	defer client.CloseIdleConnections()
	client.SetFollowRedirect(true)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, err)
	}
	httpReq.Header = http.Header{
		"accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8"},
		"accept-language":           {"en-US,en;q=0.9"},
		"cache-control":             {"max-age=0"},
		"pragma":                    {"no-cache"},
		"sec-ch-ua":                 {`"Chromium";v="120", "Not_A Brand";v="24", "Google Chrome";v="120"`},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {`"macOS"`},
		"sec-fetch-dest":            {"document"},
		"sec-fetch-mode":            {"navigate"},
		"sec-fetch-site":            {"none"},
		"sec-fetch-user":            {"?1"},
		"upgrade-insecure-requests": {"1"},
		"user-agent":                {"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
		http.HeaderOrderKey: {
			"accept",
			"accept-language",
			"cache-control",
			"pragma",
			"sec-ch-ua",
			"sec-ch-ua-mobile",
			"sec-ch-ua-platform",
			"sec-fetch-dest",
			"sec-fetch-mode",
			"sec-fetch-site",
			"sec-fetch-user",
			"upgrade-insecure-requests",
			"user-agent",
		},
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, classifyHTTPError(b.Name(), "read", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, mapStatusError(b.Name(), "read", resp.StatusCode)
	}

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, readErr)
	}

	parsedURL, parseErr := url.Parse(req.URL)
	if parseErr != nil {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, parseErr)
	}

	article, parseErr := readability.FromReader(bytes.NewReader(body), parsedURL)
	if parseErr != nil {
		return &ReadResponse{
			URL:       req.URL,
			Title:     req.URL,
			Content:   string(body),
			Backend:   b.Name(),
			FetchedAt: time.Now(),
			Metadata:  map[string]string{"mode": "raw_fallback", "readability_error": parseErr.Error()},
		}, nil
	}

	content := strings.TrimSpace(article.TextContent)
	if content == "" {
		content = strings.TrimSpace(article.Content)
	}
	title := strings.TrimSpace(article.Title)
	if title == "" {
		title = req.URL
	}

	return &ReadResponse{
		URL:       req.URL,
		Title:     title,
		Content:   content,
		Backend:   b.Name(),
		FetchedAt: time.Now(),
	}, nil
}

func (b *StealthBackend) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	_ = ctx
	_ = req
	return nil, NewBackendError(ErrParse, b.Name(), "search", false, fmt.Errorf("search unsupported for stealth backend"))
}
