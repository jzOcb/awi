package backend

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-shiori/go-readability"
)

// BrowserBackend uses Vercel's agent-browser CLI for headless browser automation.
type BrowserBackend struct {
	execPath string
	timeout  time.Duration
}

func NewBrowserBackend(timeout time.Duration) *BrowserBackend {
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return &BrowserBackend{execPath: findAgentBrowser(), timeout: timeout}
}

func (b *BrowserBackend) Name() string  { return "browser" }
func (b *BrowserBackend) Priority() int { return 3 }
func (b *BrowserBackend) Available() bool {
	return strings.TrimSpace(b.execPath) != ""
}

func (b *BrowserBackend) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
	if req == nil || strings.TrimSpace(req.URL) == "" {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, errors.New("missing URL"))
	}
	if !b.Available() {
		return nil, NewBackendError(ErrUpstream, b.Name(), "read", true, errors.New("agent-browser not found; install with: npm install -g agent-browser"))
	}

	timeout := b.timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Open the URL
	if err := b.run(runCtx, "open", req.URL); err != nil {
		return nil, NewBackendError(ErrUpstream, b.Name(), "read", true, fmt.Errorf("open: %w", err))
	}

	// Get the HTML content
	htmlContent, err := b.runOutput(runCtx, "get", "html", "html")
	if err != nil {
		_ = b.run(context.Background(), "close")
		return nil, NewBackendError(ErrUpstream, b.Name(), "read", true, fmt.Errorf("get html: %w", err))
	}

	// Get title
	title, _ := b.runOutput(runCtx, "get", "title")

	// Close browser
	_ = b.run(context.Background(), "close")

	if strings.TrimSpace(htmlContent) == "" {
		return nil, NewBackendError(ErrUpstream, b.Name(), "read", true, errors.New("empty page content"))
	}

	parsedURL, parseErr := url.Parse(req.URL)
	if parseErr != nil {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, parseErr)
	}

	article, readErr := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if readErr != nil {
		return &ReadResponse{
			URL:       req.URL,
			Title:     strings.TrimSpace(title),
			Content:   htmlContent,
			Backend:   b.Name(),
			FetchedAt: time.Now(),
			Metadata:  map[string]string{"mode": "raw_html_fallback", "readability_error": readErr.Error()},
		}, nil
	}

	content := strings.TrimSpace(article.TextContent)
	if content == "" {
		content = strings.TrimSpace(article.Content)
	}

	if len(content) < 200 && len(htmlContent) > 1000 {
		fallback := FallbackExtract(htmlContent)
		if len(fallback) > len(content) {
			content = fallback
		}
	}
	content = StripInlineTags(content)

	finalTitle := strings.TrimSpace(article.Title)
	if finalTitle == "" {
		finalTitle = strings.TrimSpace(title)
	}
	if finalTitle == "" {
		finalTitle = req.URL
	}

	return &ReadResponse{
		URL:       req.URL,
		Title:     finalTitle,
		Content:   content,
		Backend:   b.Name(),
		FetchedAt: time.Now(),
	}, nil
}

func (b *BrowserBackend) Search(_ context.Context, _ *SearchRequest) (*SearchResponse, error) {
	return nil, NewBackendError(ErrParse, b.Name(), "search", false, errors.New("search unsupported for browser backend"))
}

// Snapshot opens a URL and returns the accessibility tree with refs.
func (b *BrowserBackend) Snapshot(ctx context.Context, targetURL string) (string, error) {
	if !b.Available() {
		return "", fmt.Errorf("agent-browser not found")
	}
	if err := b.run(ctx, "open", targetURL); err != nil {
		return "", fmt.Errorf("open: %w", err)
	}
	out, err := b.runOutput(ctx, "snapshot")
	if err != nil {
		_ = b.run(context.Background(), "close")
		return "", fmt.Errorf("snapshot: %w", err)
	}
	return out, nil
}

// Act executes an agent-browser command (click, fill, type, etc.)
func (b *BrowserBackend) Act(ctx context.Context, args ...string) (string, error) {
	if !b.Available() {
		return "", fmt.Errorf("agent-browser not found")
	}
	return b.runOutput(ctx, args...)
}

// Close closes the browser.
func (b *BrowserBackend) Close() error {
	return b.run(context.Background(), "close")
}

func (b *BrowserBackend) run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, b.execPath, args...)
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *BrowserBackend) runOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, b.execPath, args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%w: %s", err, string(exitErr.Stderr))
		}
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func findAgentBrowser() string {
	if p, err := exec.LookPath("agent-browser"); err == nil {
		return p
	}
	return ""
}
