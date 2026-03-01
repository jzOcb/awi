package backend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
	
)

type DirectBackend struct {
	timeout time.Duration
}

func NewDirectBackend(timeout time.Duration) *DirectBackend {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &DirectBackend{timeout: timeout}
}

func (b *DirectBackend) Name() string    { return "direct" }
func (b *DirectBackend) Priority() int   { return 1 }
func (b *DirectBackend) Available() bool { return true }

func (b *DirectBackend) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
	if req == nil || strings.TrimSpace(req.URL) == "" {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, errors.New("missing URL"))
	}

	timeout := b.timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}
	client, err := newHTTPClientWithProxy(timeout, req.Proxy)
	if err != nil {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, err)
	}
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	httpReq.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	httpReq.Header.Set("Accept-Language", "en-US,en;q=0.9")

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

	article, err := readability.FromReader(bytes.NewReader(body), parsedURL)
	if err != nil {
		return &ReadResponse{
			URL:       req.URL,
			Title:     req.URL,
			Content:   string(body),
			Backend:   b.Name(),
			FetchedAt: time.Now(),
			Metadata:  map[string]string{"mode": "raw_fallback", "readability_error": err.Error()},
		}, nil
	}

	content := strings.TrimSpace(article.TextContent)
	if content == "" {
		content = strings.TrimSpace(article.Content)
	}
	content = StripInlineTags(content)

	// Fallback: if readability output is suspiciously short but raw HTML is long,
	// readability likely failed to identify the main content.
	if len(content) < 200 && len(body) > 1000 {
		fallback := FallbackExtract(string(body))
		if len(fallback) > len(content) {
			content = fallback
		}
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

func (b *DirectBackend) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	_ = ctx
	_ = req
	return nil, NewBackendError(ErrParse, b.Name(), "search", false, errors.New("search unsupported for direct backend"))
}
