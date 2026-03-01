package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/jasonz/webscout/internal/backend"
	"github.com/jasonz/webscout/internal/cache"
	"github.com/jasonz/webscout/internal/config"
	"github.com/jasonz/webscout/internal/retry"
)

var (
	backendFlag string
	formatFlag  string
	proxyFlag   string
	noCacheFlag bool
	timeoutFlag time.Duration

	cfg         *config.Config
	router      *backend.Router
	diskCache   *cache.DiskCache
	retryPolicy retry.Policy
)

var rootCmd = &cobra.Command{
	Use:   "ws",
	Short: "webscout multi-backend web fetcher",
	PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
		if cfg != nil {
			return nil
		}
		loaded, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		cfg = loaded

		if strings.TrimSpace(formatFlag) == "" {
			formatFlag = cfg.Output.DefaultFormat
		}

		proxyFlag = resolveProxyValue(proxyFlag, cfg.Network.Proxy)

		if !noCacheFlag && cfg.Cache.Enabled {
			c, err := cache.New(cfg.Cache.Dir)
			if err != nil {
				return fmt.Errorf("init cache: %w", err)
			}
			diskCache = c
		}

		retryPolicy = retry.Policy{
			MaxAttempts:  cfg.Retry.MaxAttempts,
			Multiplier:   cfg.Retry.Multiplier,
			InitialDelay: parseDurationOr(cfg.Retry.InitialDelay, 500*time.Millisecond),
			MaxDelay:     parseDurationOr(cfg.Retry.MaxDelay, 5*time.Second),
		}

		backends := make([]backend.Backend, 0, 3)
		if bcfg, ok := cfg.Backends["direct"]; ok && bcfg.Enabled {
			backends = append(backends, backend.NewDirectBackend(parseDurationOr(bcfg.Timeout, 20*time.Second)))
		}
		if bcfg, ok := cfg.Backends["stealth"]; ok && bcfg.Enabled {
			backends = append(backends, backend.NewStealthBackend(parseDurationOr(bcfg.Timeout, 30*time.Second)))
		}
		if bcfg, ok := cfg.Backends["browser"]; ok && bcfg.Enabled {
			backends = append(backends, backend.NewBrowserBackend(parseDurationOr(bcfg.Timeout, 45*time.Second)))
		}
		if len(backends) == 0 {
			return fmt.Errorf("no enabled backends")
		}

		router = backend.NewRouter(backends, backend.RouterOptions{})
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&backendFlag, "backend", "", "preferred backend (direct,stealth,browser)")
	rootCmd.PersistentFlags().StringVar(&formatFlag, "format", "", "output format: json|markdown|text")
	rootCmd.PersistentFlags().StringVar(&proxyFlag, "proxy", "", "proxy URL for read backends (http://..., socks5://...)")
	rootCmd.PersistentFlags().BoolVar(&noCacheFlag, "no-cache", false, "disable cache")
	rootCmd.PersistentFlags().DurationVar(&timeoutFlag, "timeout", 30*time.Second, "request timeout")

	rootCmd.AddCommand(newReadCmd())
	rootCmd.AddCommand(newSearchCmd())
	rootCmd.AddCommand(newBatchCmd())
}

func commandContext() (context.Context, context.CancelFunc) {
	if timeoutFlag <= 0 {
		return context.WithCancel(context.Background())
	}
	return context.WithTimeout(context.Background(), timeoutFlag)
}

func parseDurationOr(input string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(strings.TrimSpace(input))
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

func resolveProxyValue(cliValue, configValue string) string {
	if v := strings.TrimSpace(cliValue); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("WEBSCOUT_PROXY")); v != "" {
		return v
	}
	return strings.TrimSpace(configValue)
}
