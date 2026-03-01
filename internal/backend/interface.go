package backend

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type ErrorCode string

const (
	ErrAuth      ErrorCode = "auth"
	ErrRateLimit ErrorCode = "rate_limit"
	ErrParse     ErrorCode = "parse"
	ErrUpstream  ErrorCode = "upstream"
	ErrTimeout   ErrorCode = "timeout"
)

type BackendError struct {
	Code      ErrorCode
	Backend   string
	Operation string
	Retryable bool
	Err       error
}

func (e *BackendError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("backend=%s op=%s code=%s: %v", e.Backend, e.Operation, e.Code, e.Err)
}

func (e *BackendError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *BackendError) IsRetryable() bool {
	if e == nil {
		return false
	}
	return e.Retryable
}

func NewBackendError(code ErrorCode, backend, operation string, retryable bool, err error) error {
	return &BackendError{Code: code, Backend: backend, Operation: operation, Retryable: retryable, Err: err}
}

func IsCode(err error, code ErrorCode) bool {
	var be *BackendError
	if errors.As(err, &be) {
		return be.Code == code
	}
	return false
}

func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}
	var be *BackendError
	if errors.As(err, &be) {
		return be.IsRetryable()
	}
	return false
}

type Backend interface {
	Name() string
	Priority() int
	Available() bool
	Read(ctx context.Context, req *ReadRequest) (*ReadResponse, error)
	Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error)
}

type ReadRequest struct {
	URL     string
	Timeout time.Duration
	Proxy   string
	Options map[string]string
}

type SearchRequest struct {
	Query   string
	Limit   int
	Timeout time.Duration
	Proxy   string
	Options map[string]string
}

type ReadResponse struct {
	URL       string            `json:"url" yaml:"url"`
	Title     string            `json:"title" yaml:"title"`
	Content   string            `json:"content" yaml:"content"`
	Backend   string            `json:"backend" yaml:"backend"`
	FetchedAt time.Time         `json:"fetched_at" yaml:"fetched_at"`
	Metadata  map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CacheHit  bool              `json:"cache_hit" yaml:"cache_hit"`
}

type SearchResult struct {
	Title   string `json:"title" yaml:"title"`
	URL     string `json:"url" yaml:"url"`
	Snippet string `json:"snippet" yaml:"snippet"`
}

type SearchResponse struct {
	Query     string         `json:"query" yaml:"query"`
	Results   []SearchResult `json:"results" yaml:"results"`
	Limit     int            `json:"limit" yaml:"limit"`
	Backend   string         `json:"backend" yaml:"backend"`
	FetchedAt time.Time      `json:"fetched_at" yaml:"fetched_at"`
	CacheHit  bool           `json:"cache_hit" yaml:"cache_hit"`
}
