# AWI

**Agentic Web Interface** — A zero-config, single-binary web content fetcher for AI agents. No API keys, no external services, no dependencies.

```bash
# Read a webpage
awi read https://example.com

# Search the web
awi search "Claude API rate limits" --limit 5

# Batch fetch
awi batch urls.txt --concurrency 10
```

## Why awi?

AI agents need to read the web. Existing solutions either:
- **Require API keys** (Jina Reader, Firecrawl) — costs money, rate limits, privacy concerns
- **Require deploying services** (Firecrawl self-hosted) — complex setup
- **Are Python-only** (Crawl4AI) — slow startup, dependency hell

awi is a **single Go binary** that works out of the box:

| | awi | Jina Reader | Firecrawl | Crawl4AI |
|---|---|---|---|---|
| API key required | ❌ | ✅ | ✅ | ❌ |
| Install | Single binary | `go install` | Docker/deploy | `pip install` |
| Anti-bot bypass | ✅ Built-in | Cloud proxy | Cloud proxy | Basic |
| Offline capable | ✅ | ❌ | ❌ | ✅ |
| Language | Go | Go | Python | Python |

## Install

### From source
```bash
go install github.com/jzOcb/awi/cmd/awi@latest
```

### From binary
Download from [Releases](https://github.com/jzOcb/awi/releases).

### Homebrew (coming soon)
```bash
brew install jzOcb/tap/awi
```

## How it works

awi uses a **3-tier backend architecture** with automatic escalation:

```
awi read <url>
  │
  ├─ 1. direct    → Plain HTTP + readability (fastest, zero overhead)
  │     ↓ 403?
  ├─ 2. stealth   → Chrome TLS fingerprint via tls-client (bypasses Cloudflare)
  │     ↓ still blocked?
  └─ 3. browser   → Headless Chrome with anti-detection (handles JS-heavy pages)
```

No configuration needed. If a simple HTTP request works, it uses that. If the site has bot protection, it automatically escalates.

## Usage

### Read a webpage
```bash
# Default (auto-selects best backend)
awi read https://docs.python.org/3/tutorial/

# Force a specific backend
awi read https://example.com --backend direct
awi read https://spa-app.com --backend browser
awi read https://protected-site.com --backend stealth

# Force JS rendering
awi read https://react-app.com --js

# Output formats
awi read https://example.com --format json      # default
awi read https://example.com --format markdown
awi read https://example.com --format text
```

### Search the web
```bash
# Search via DuckDuckGo (no API key needed)
awi search "golang web scraping" --limit 5
```

### Batch fetch
```bash
# From file
awi batch urls.txt --concurrency 10

# From stdin
cat urls.txt | awi batch - --concurrency 5
```

### Proxy support
```bash
awi read https://example.com --proxy http://user:pass@proxy:8080
awi read https://example.com --proxy socks5://127.0.0.1:1080
```

## Output format

### JSON (default)
```json
{
  "url": "https://example.com",
  "title": "Example Domain",
  "content": "This domain is for use in illustrative examples...",
  "backend": "direct",
  "fetched_at": "2026-02-28T20:43:46Z",
  "cache_hit": false
}
```

### Markdown
```markdown
# Example Domain

Source: https://example.com
Backend: direct

This domain is for use in illustrative examples...
```

## Configuration

Optional. Create `~/.awi/config.yaml`:

```yaml
# Default output format
format: json

# Default timeout
timeout: 30s

# Cache settings
cache:
  enabled: true
  ttl: 24h
  dir: ~/.awi/cache

# Network settings
network:
  proxy: ""  # default proxy for all requests
```

## Backend details

### direct
- Pure HTTP with realistic browser headers
- Content extraction via [go-readability](https://github.com/go-shiori/go-readability)
- Fallback text extraction for complex pages
- **Best for:** Static HTML, blogs, documentation, news

### stealth
- Uses [tls-client](https://github.com/bogdanfinn/tls-client) to mimic Chrome 120 TLS fingerprint
- Bypasses basic Cloudflare and bot detection
- No browser needed — pure HTTP with browser-like transport
- **Best for:** Sites with Cloudflare or basic bot protection

### browser
- Headless Chrome via [chromedp](https://github.com/chromedp/chromedp)
- Anti-detection: removes `navigator.webdriver`, disables automation flags
- Waits for page load + network idle
- **Best for:** JavaScript SPAs, dynamic content, heavy anti-bot pages

## Caching

awi caches responses locally (default 24h TTL):
- Cache dir: `~/.awi/cache/`
- SHA256-keyed JSON files
- Cache keys include URL + backend + options
- Disable with `--no-cache`
- File permissions: `0600` (private)

## For AI agent developers

awi is designed to be called by AI agents:

```python
import subprocess
import json

result = subprocess.run(
    ["awi", "read", url, "--format", "json", "--no-cache"],
    capture_output=True, text=True
)
data = json.loads(result.stdout)
content = data["content"]
```

Or use it as an [OpenClaw](https://github.com/openclaw/openclaw) skill — SKILL.md coming soon.

## Test results

Tested against 30 diverse websites across 8 categories:

```
Total: 30 | Pass: 27 | Fail: 2 | Flaky: 1
Score: 116/120 (96.7%)

By category:
  static:       100%
  tech_docs:     90%
  news_blog:    100%
  github:       100%
  chinese:      100%
  social_forum: 100%
  cloudflare:    83%
  edge_cases:   100%
```

The 2 failures are sites with aggressive enterprise-grade bot protection (OpenAI, Cloudflare.com) that require residential proxy pools to access — a limitation shared by all local CLI tools.

## Roadmap

- [ ] OpenClaw SKILL.md
- [ ] Homebrew formula
- [ ] `awi extract` — LLM-powered structured data extraction
- [ ] Residential proxy pool integration
- [ ] Cookie/session management
- [ ] PDF/document parsing
- [ ] GitHub Actions for cross-platform builds

## License

MIT

## Credits

Built with:
- [cobra](https://github.com/spf13/cobra) — CLI framework
- [go-readability](https://github.com/go-shiori/go-readability) — Content extraction
- [tls-client](https://github.com/bogdanfinn/tls-client) — Browser TLS fingerprinting
- [chromedp](https://github.com/chromedp/chromedp) — Headless Chrome
- [goquery](https://github.com/PuerkitoBio/goquery) — HTML parsing
