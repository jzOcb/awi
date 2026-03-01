package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type entry struct {
	Key       string          `json:"key"`
	CreatedAt time.Time       `json:"created_at"`
	ExpiresAt time.Time       `json:"expires_at"`
	Value     json.RawMessage `json:"value"`
}

type DiskCache struct {
	dir string
	mu  sync.Mutex
}

var strictHexKey = regexp.MustCompile(`^[a-f0-9]+$`)

func New(dir string) (*DiskCache, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("cache dir is required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &DiskCache{dir: dir}, nil
}

func BuildKey(base, format, backend string, opts map[string]string) string {
	parts := []string{base, format, backend}
	keys := make([]string, 0, len(opts))
	for k := range opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, k+"="+opts[k])
	}
	raw := strings.Join(parts, "|")
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func (c *DiskCache) Get(key string, out any) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	path, err := c.filePath(key)
	if err != nil {
		return false, err
	}
	blob, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	var e entry
	if err := json.Unmarshal(blob, &e); err != nil {
		return false, err
	}
	if time.Now().After(e.ExpiresAt) {
		_ = os.Remove(path)
		return false, nil
	}
	if err := json.Unmarshal(e.Value, out); err != nil {
		return false, err
	}
	return true, nil
}

func (c *DiskCache) Set(key string, ttl time.Duration, value any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ttl <= 0 {
		return nil
	}
	path, err := c.filePath(key)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	e := entry{
		Key:       key,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		Value:     payload,
	}
	blob, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, blob, 0o600)
}

func (c *DiskCache) filePath(key string) (string, error) {
	if !strictHexKey.MatchString(key) {
		return "", fmt.Errorf("invalid cache key")
	}
	return filepath.Join(c.dir, key+".json"), nil
}
