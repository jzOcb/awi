package backend

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/go-shiori/go-readability"
	
)

type BrowserBackend struct {
	execPath string
	timeout  time.Duration
}

func NewBrowserBackend(timeout time.Duration) *BrowserBackend {
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	return &BrowserBackend{execPath: findChromeExecutable(), timeout: timeout}
}

func (b *BrowserBackend) Name() string  { return "browser" }
func (b *BrowserBackend) Priority() int { return 3 }
func (b *BrowserBackend) Available() bool {
	if strings.TrimSpace(b.execPath) == "" {
		return false
	}
	_, err := os.Stat(b.execPath)
	return err == nil
}

func (b *BrowserBackend) Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error) {
	if req == nil || strings.TrimSpace(req.URL) == "" {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, errors.New("missing URL"))
	}
	if !b.Available() {
		return nil, NewBackendError(ErrUpstream, b.Name(), "read", true, errors.New("chrome executable not found"))
	}

	timeout := b.timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	allocOptions := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(b.execPath),
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("window-size", "1366,768"),
		chromedp.Flag("lang", "en-US"),
	}
	if proxyURL := strings.TrimSpace(req.Proxy); proxyURL != "" {
		allocOptions = append(allocOptions, chromedp.Flag("proxy-server", proxyURL))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOptions...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	runCtx, cancelRun := context.WithTimeout(browserCtx, timeout)
	defer cancelRun()

	htmlContent, title, err := fetchPageHTML(runCtx, req.URL)
	if err != nil {
		return nil, NewBackendError(ErrUpstream, b.Name(), "read", true, err)
	}

	parsedURL, parseErr := url.Parse(req.URL)
	if parseErr != nil {
		return nil, NewBackendError(ErrParse, b.Name(), "read", false, parseErr)
	}

	article, readErr := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if readErr != nil {
		return &ReadResponse{
			URL:       req.URL,
			Title:     title,
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

	// Fallback: if readability output is suspiciously short but raw HTML is long,
	// use simple text extraction from HTML.
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

func (b *BrowserBackend) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	_ = ctx
	_ = req
	return nil, NewBackendError(ErrParse, b.Name(), "search", false, errors.New("search unsupported for browser backend"))
}

func fetchPageHTML(ctx context.Context, targetURL string) (string, string, error) {
	var htmlContent string
	var title string

	if err := chromedp.Run(ctx,
		network.Enable(),
		page.Enable(),
		injectStealthScript(),
		chromedp.Navigate(targetURL),
		waitForLoadEvent(15*time.Second),
		waitForNetworkIdleOrDelay(700*time.Millisecond, 5*time.Second, 3*time.Second),
		chromedp.Title(&title),
		chromedp.OuterHTML("html", &htmlContent, chromedp.ByQuery),
	); err != nil {
		return "", "", err
	}

	if strings.TrimSpace(htmlContent) == "" {
		return "", "", io.EOF
	}
	return htmlContent, title, nil
}

func injectStealthScript() chromedp.Action {
	const script = `(function () {
  Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
  Object.defineProperty(navigator, 'plugins', { get: () => [1, 2, 3, 4] });
})();`

	return chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(script).Do(ctx)
		return err
	})
}

func waitForNetworkIdle(idleFor, maxWait time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var mu sync.Mutex
		inflight := 0
		lastActivity := time.Now()
		loaded := false

		chromedp.ListenTarget(ctx, func(ev interface{}) {
			mu.Lock()
			defer mu.Unlock()
			switch ev.(type) {
			case *network.EventRequestWillBeSent:
				inflight++
				lastActivity = time.Now()
			case *network.EventLoadingFinished, *network.EventLoadingFailed:
				if inflight > 0 {
					inflight--
				}
				lastActivity = time.Now()
			case *page.EventLoadEventFired:
				loaded = true
				lastActivity = time.Now()
			}
		})

		deadline := time.Now().Add(maxWait)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				now := time.Now()
				mu.Lock()
				idle := loaded && inflight == 0 && now.Sub(lastActivity) >= idleFor
				mu.Unlock()
				if idle {
					return nil
				}
				if now.After(deadline) {
					return errors.New("network idle timeout")
				}
			}
		}
	})
}

func waitForLoadEvent(maxWait time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		done := make(chan struct{})
		var once sync.Once

		chromedp.ListenTarget(ctx, func(ev interface{}) {
			switch ev.(type) {
			case *page.EventLoadEventFired:
				once.Do(func() { close(done) })
			}
		})

		timer := time.NewTimer(maxWait)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer timer.Stop()
		defer ticker.Stop()

		for {
			var state string
			if err := chromedp.Evaluate(`document.readyState`, &state).Do(ctx); err == nil {
				state = strings.ToLower(strings.TrimSpace(state))
				if state == "interactive" || state == "complete" {
					return nil
				}
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-done:
				return nil
			case <-timer.C:
				return errors.New("page load event timeout")
			case <-ticker.C:
			}
		}
	})
}

func waitForNetworkIdleOrDelay(idleFor, idleMaxWait, fallbackDelay time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		idleDone := make(chan struct{}, 1)

		waitCtx, cancel := context.WithTimeout(ctx, idleMaxWait)
		defer cancel()

		go func() {
			_ = waitForNetworkIdle(idleFor, idleMaxWait).Do(waitCtx)
			idleDone <- struct{}{}
		}()

		timer := time.NewTimer(fallbackDelay)
		defer timer.Stop()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-idleDone:
			return nil
		case <-timer.C:
			return nil
		}
	})
}

func findChromeExecutable() string {
	candidates := []string{
		"/usr/bin/google-chrome",
		"/usr/bin/google-chrome-stable",
		"/usr/bin/chromium",
		"/usr/bin/chromium-browser",
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
		"/snap/bin/chromium",
	}
	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}

	for _, bin := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"} {
		if resolved, err := exec.LookPath(bin); err == nil {
			return resolved
		}
	}
	return ""
}
