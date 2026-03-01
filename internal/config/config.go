package config

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Backends map[string]BackendConfig `yaml:"backends"`
	Cache    CacheConfig              `yaml:"cache"`
	Output   OutputConfig             `yaml:"output"`
	Retry    RetryConfig              `yaml:"retry"`
	Network  NetworkConfig            `yaml:"network"`
}

type BackendConfig struct {
	Enabled bool   `yaml:"enabled"`
	Timeout string `yaml:"timeout"`
}

type CacheConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Dir       string `yaml:"dir"`
	ReadTTL   string `yaml:"read_ttl"`
	SearchTTL string `yaml:"search_ttl"`
}

type OutputConfig struct {
	DefaultFormat     string `yaml:"default_format"`
	MarkdownMaxLength int    `yaml:"markdown_max_length"`
}

type RetryConfig struct {
	MaxAttempts  int     `yaml:"max_attempts"`
	InitialDelay string  `yaml:"initial_delay"`
	Multiplier   float64 `yaml:"multiplier"`
	MaxDelay     string  `yaml:"max_delay"`
}

type NetworkConfig struct {
	Proxy string `yaml:"proxy"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	cacheDir := filepath.Join(home, ".webscout", "cache")
	return &Config{
		Backends: map[string]BackendConfig{
			"direct":  {Enabled: true, Timeout: "20s"},
			"stealth": {Enabled: true, Timeout: "30s"},
			"browser": {Enabled: true, Timeout: "45s"},
		},
		Cache:   CacheConfig{Enabled: true, Dir: cacheDir, ReadTTL: "24h", SearchTTL: "1h"},
		Output:  OutputConfig{DefaultFormat: "json", MarkdownMaxLength: 0},
		Retry:   RetryConfig{MaxAttempts: 3, InitialDelay: "500ms", Multiplier: 2, MaxDelay: "5s"},
		Network: NetworkConfig{Proxy: ""},
	}
}

func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".webscout", "config.yaml"), nil
}

func Load() (*Config, error) {
	cfg := Default()
	path, err := Path()
	if err != nil {
		return nil, err
	}

	if blob, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(blob, cfg); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	applyEnv(cfg)
	if err := normalize(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) ReadTTL() time.Duration {
	d, _ := time.ParseDuration(c.Cache.ReadTTL)
	if d <= 0 {
		d = 24 * time.Hour
	}
	return d
}

func (c *Config) SearchTTL() time.Duration {
	d, _ := time.ParseDuration(c.Cache.SearchTTL)
	if d <= 0 {
		d = time.Hour
	}
	return d
}

func (c *Config) BackendTimeout(name string) time.Duration {
	bc, ok := c.Backends[name]
	if !ok {
		return 30 * time.Second
	}
	d, err := time.ParseDuration(strings.TrimSpace(bc.Timeout))
	if err != nil || d <= 0 {
		return 30 * time.Second
	}
	return d
}

func applyEnv(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("WEBSCOUT_DEFAULT_FORMAT")); v != "" {
		cfg.Output.DefaultFormat = v
	}
	if v := strings.TrimSpace(os.Getenv("WEBSCOUT_MARKDOWN_MAX_LENGTH")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Output.MarkdownMaxLength = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("WEBSCOUT_CACHE_DIR")); v != "" {
		cfg.Cache.Dir = v
	}
	if v := strings.TrimSpace(os.Getenv("WEBSCOUT_PROXY")); v != "" {
		cfg.Network.Proxy = v
	}
}

func normalize(cfg *Config) error {
	if cfg.Backends == nil {
		cfg.Backends = map[string]BackendConfig{}
	}
	if _, ok := cfg.Backends["direct"]; !ok {
		cfg.Backends["direct"] = BackendConfig{Enabled: true, Timeout: "20s"}
	}
	if _, ok := cfg.Backends["stealth"]; !ok {
		cfg.Backends["stealth"] = BackendConfig{Enabled: true, Timeout: "30s"}
	}
	if _, ok := cfg.Backends["browser"]; !ok {
		cfg.Backends["browser"] = BackendConfig{Enabled: true, Timeout: "45s"}
	}
	if strings.TrimSpace(cfg.Output.DefaultFormat) == "" {
		cfg.Output.DefaultFormat = "json"
	}
	if strings.TrimSpace(cfg.Cache.Dir) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		cfg.Cache.Dir = filepath.Join(home, ".webscout", "cache")
	}
	cfg.Network.Proxy = strings.TrimSpace(cfg.Network.Proxy)
	return nil
}
