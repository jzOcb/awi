package cmd

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
	xproxy "golang.org/x/net/proxy"

	"github.com/jasonz/webscout/internal/backend"
	"github.com/jasonz/webscout/internal/cache"
	"github.com/jasonz/webscout/internal/output"
)

func newSearchCmd() *cobra.Command {
	var limit int
	c := &cobra.Command{
		Use:   "search <query>",
		Short: "Search DuckDuckGo HTML (no API key required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			cacheKey := cache.BuildKey("search:"+query, formatFlag, "duckduckgo", map[string]string{"limit": intToString(limit), "timeout": timeoutFlag.String()})

			if diskCache != nil {
				var cached backend.SearchResponse
				hit, err := diskCache.Get(cacheKey, &cached)
				if err != nil {
					return err
				}
				if hit {
					cached.CacheHit = true
					return printSearchOutput(&cached)
				}
			}

			ctx, cancel := commandContext()
			defer cancel()

			resp, err := searchDuckDuckGoHTML(ctx, query, limit, timeoutFlag, proxyFlag)
			if err != nil {
				return err
			}

			if diskCache != nil {
				if err := diskCache.Set(cacheKey, cfg.SearchTTL(), resp); err != nil {
					fmt.Fprintf(os.Stderr, "warning: cache write failed: %v\n", err)
				}
			}
			return printSearchOutput(resp)
		},
	}
	c.Flags().IntVar(&limit, "limit", 10, "maximum number of results")
	return c
}

func printSearchOutput(v any) error {
	rendered, err := output.Render(formatFlag, v, cfg.Output.MarkdownMaxLength)
	if err != nil {
		return err
	}
	fmt.Println(rendered)
	return nil
}

type contextDialer interface {
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

func newSearchHTTPClientWithProxy(timeout time.Duration, proxyRaw string) (*http.Client, error) {
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

func searchDuckDuckGoHTML(ctx context.Context, query string, limit int, timeout time.Duration, proxyRaw string) (*backend.SearchResponse, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("missing query")
	}
	if limit <= 0 {
		limit = 10
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	target := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; webscout/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	client, err := newSearchHTTPClientWithProxy(timeout, proxyRaw)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("duckduckgo status %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	results := make([]backend.SearchResult, 0, limit)
	doc.Find("div.result").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if len(results) >= limit {
			return false
		}
		link := s.Find("a.result__a").First()
		href, _ := link.Attr("href")
		if strings.HasPrefix(href, "//") {
			href = "https:" + href
		}
		if strings.Contains(href, "uddg=") {
			if u, err := url.Parse(href); err == nil {
				if actual := u.Query().Get("uddg"); actual != "" {
					href = actual
				}
			}
		}
		title := strings.TrimSpace(link.Text())
		snippet := strings.TrimSpace(s.Find(".result__snippet").First().Text())

		if title == "" || href == "" {
			return true
		}
		results = append(results, backend.SearchResult{Title: title, URL: href, Snippet: snippet})
		return true
	})

	if len(results) == 0 {
		return nil, fmt.Errorf("no search results parsed")
	}

	return &backend.SearchResponse{
		Query:     query,
		Results:   results,
		Limit:     limit,
		Backend:   "duckduckgo",
		FetchedAt: time.Now(),
	}, nil
}
