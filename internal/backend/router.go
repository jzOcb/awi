package backend

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

type RouterOptions struct {
	FailureWindow    time.Duration
	FailureThreshold int
	Cooldown         time.Duration
}

type backendHealth struct {
	failures      []time.Time
	cooldownUntil time.Time
}

type Router struct {
	backends []Backend
	opts     RouterOptions
	mu       sync.Mutex
	health   map[string]*backendHealth
	domainOK map[string]string
}

func NewRouter(backends []Backend, opts RouterOptions) *Router {
	if opts.FailureWindow <= 0 {
		opts.FailureWindow = 2 * time.Minute
	}
	if opts.FailureThreshold <= 0 {
		opts.FailureThreshold = 3
	}
	if opts.Cooldown <= 0 {
		opts.Cooldown = 1 * time.Minute
	}
	copied := append([]Backend(nil), backends...)
	sort.SliceStable(copied, func(i, j int) bool { return copied[i].Priority() < copied[j].Priority() })

	health := make(map[string]*backendHealth, len(copied))
	for _, b := range copied {
		health[b.Name()] = &backendHealth{}
	}

	return &Router{backends: copied, opts: opts, health: health, domainOK: make(map[string]string)}
}

func (r *Router) Read(ctx context.Context, preferred string, req *ReadRequest) (*ReadResponse, error) {
	pinned := preferred != "" // user explicitly chose a backend — no escalation allowed
	var errs []error
	attempted := make(map[string]bool)

	for _, b := range r.candidates(preferred, reqURL(req)) {
		if attempted[b.Name()] {
			continue
		}
		attempted[b.Name()] = true
		if !b.Available() || !r.isHealthy(b.Name()) {
			continue
		}

		resp, err := b.Read(ctx, req)
		if err == nil {
			if !pinned && shouldEscalateToBrowser(b.Name(), resp) {
				browser := r.backendByName("browser")
				if browser != nil && !attempted["browser"] && browser.Available() && r.isHealthy(browser.Name()) {
					attempted["browser"] = true
					browserResp, browserErr := browser.Read(ctx, req)
					if browserErr == nil {
						r.markSuccess(browser.Name())
						r.cacheDomainBackend(req.URL, browser.Name())
						browserResp.Backend = browser.Name()
						return browserResp, nil
					}
					errs = append(errs, browserErr)
					r.markFailure(browser.Name(), browserErr)
				}
				// Challenge-like content should not be treated as success; continue trying other backends.
				errs = append(errs, NewBackendError(ErrUpstream, b.Name(), "read", true, errors.New("challenge content detected")))
				r.markFailure(b.Name(), errs[len(errs)-1])
				continue
			}

			r.markSuccess(b.Name())
			r.cacheDomainBackend(req.URL, b.Name())
			resp.Backend = b.Name()
			return resp, nil
		}

		errs = append(errs, err)
		r.markFailure(b.Name(), err)

		if !pinned && b.Name() == "direct" && shouldEscalateToStealth(err) {
			stealth := r.backendByName("stealth")
			if stealth != nil && !attempted["stealth"] && stealth.Available() && r.isHealthy(stealth.Name()) {
				attempted["stealth"] = true
				stealthResp, stealthErr := stealth.Read(ctx, req)
				if stealthErr == nil {
					r.markSuccess(stealth.Name())
					r.cacheDomainBackend(req.URL, stealth.Name())
					stealthResp.Backend = stealth.Name()
					return stealthResp, nil
				}
				errs = append(errs, stealthErr)
				r.markFailure(stealth.Name(), stealthErr)
			}
		}
	}

	if len(errs) == 0 {
		return nil, errors.New("no backend available")
	}
	return nil, errors.Join(errs...)
}

func (r *Router) Search(ctx context.Context, preferred string, req *SearchRequest) (*SearchResponse, error) {
	var errs []error
	for _, b := range r.candidates(preferred, "") {
		if !b.Available() || !r.isHealthy(b.Name()) {
			continue
		}
		resp, err := b.Search(ctx, req)
		if err == nil {
			r.markSuccess(b.Name())
			resp.Backend = b.Name()
			return resp, nil
		}
		errs = append(errs, err)
		r.markFailure(b.Name(), err)
	}
	if len(errs) == 0 {
		return nil, errors.New("no backend available")
	}
	return nil, errors.Join(errs...)
}

func (r *Router) candidates(preferred, rawURL string) []Backend {
	if preferred != "" {
		// User explicitly pinned a backend — return ONLY that backend, no fallback
		for _, b := range r.backends {
			if b.Name() == preferred {
				return []Backend{b}
			}
		}
		return nil // unknown backend name
	}

	cached := r.cachedBackendForURL(rawURL)
	if cached == "" {
		return append([]Backend(nil), r.backends...)
	}

	out := make([]Backend, 0, len(r.backends))
	for _, b := range r.backends {
		if b.Name() == cached {
			out = append(out, b)
		}
	}
	for _, b := range r.backends {
		if b.Name() != cached {
			out = append(out, b)
		}
	}
	return out
}

func (r *Router) backendByName(name string) Backend {
	for _, b := range r.backends {
		if b.Name() == name {
			return b
		}
	}
	return nil
}

func (r *Router) isHealthy(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.health[name]
	if !ok {
		return true
	}
	now := time.Now()
	if now.Before(s.cooldownUntil) {
		return false
	}
	trimmed := s.failures[:0]
	for _, t := range s.failures {
		if now.Sub(t) <= r.opts.FailureWindow {
			trimmed = append(trimmed, t)
		}
	}
	s.failures = trimmed
	return len(s.failures) < r.opts.FailureThreshold
}

func (r *Router) markSuccess(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.health[name]; ok {
		s.failures = nil
		s.cooldownUntil = time.Time{}
	}
}

func (r *Router) markFailure(name string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.health[name]
	if !ok {
		return
	}
	now := time.Now()
	s.failures = append(s.failures, now)

	trimmed := s.failures[:0]
	for _, t := range s.failures {
		if now.Sub(t) <= r.opts.FailureWindow {
			trimmed = append(trimmed, t)
		}
	}
	s.failures = trimmed
	if len(s.failures) >= r.opts.FailureThreshold && IsRetryableError(err) {
		s.cooldownUntil = now.Add(r.opts.Cooldown)
	}
}

func (r *Router) cacheDomainBackend(rawURL, backend string) {
	host := extractHost(rawURL)
	if host == "" || strings.TrimSpace(backend) == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.domainOK[host] = backend
}

func (r *Router) cachedBackendForURL(rawURL string) string {
	host := extractHost(rawURL)
	if host == "" {
		return ""
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.domainOK[host]
}

func reqURL(req *ReadRequest) string {
	if req == nil {
		return ""
	}
	return req.URL
}

func extractHost(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

func shouldEscalateToStealth(err error) bool {
	var be *BackendError
	if !errors.As(err, &be) {
		return false
	}
	return be.Code == ErrAuth || be.Code == ErrRateLimit
}

func shouldEscalateToBrowser(backend string, resp *ReadResponse) bool {
	if backend == "browser" || resp == nil {
		return false
	}
	body := strings.TrimSpace(resp.Content)
	if body == "" {
		return true
	}
	// Non-browser backends often return shell/interstitial pages; treat very short content as suspicious.
	if len([]rune(body)) < 300 {
		return true
	}
	l := strings.ToLower(body)
	keywords := []string{
		"attention required",
		"cloudflare",
		"checking your browser",
		"just a moment",
		"cf-challenge",
		"enable javascript",
		"captcha",
		"verify you are human",
		"bot detection",
	}
	for _, k := range keywords {
		if strings.Contains(l, k) {
			return true
		}
	}
	return false
}
