package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jasonz/webscout/internal/backend"
	"github.com/jasonz/webscout/internal/cache"
	"github.com/jasonz/webscout/internal/output"
	"github.com/jasonz/webscout/internal/retry"
)

func newReadCmd() *cobra.Command {
	var jsFlag bool
	c := &cobra.Command{
		Use:   "read <url>",
		Short: "Read and extract main content from a URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			preferredBackend := backendFlag
			if jsFlag {
				preferredBackend = "browser"
			}
			cacheKey := cache.BuildKey("read:"+url, formatFlag, preferredBackend, map[string]string{"timeout": timeoutFlag.String(), "js": strconv.FormatBool(jsFlag), "proxy": proxyFlag})

			if diskCache != nil {
				var cached backend.ReadResponse
				hit, err := diskCache.Get(cacheKey, &cached)
				if err != nil {
					return err
				}
				if hit {
					cached.CacheHit = true
					return printOutput(&cached)
				}
			}

			ctx, cancel := commandContext()
			defer cancel()

			resp, err := retry.DoValue(ctx, retryPolicy, func() (*backend.ReadResponse, error) {
				return router.Read(ctx, preferredBackend, &backend.ReadRequest{URL: url, Timeout: timeoutFlag, Proxy: proxyFlag})
			})
			if err != nil {
				return err
			}

			if diskCache != nil {
				if err := diskCache.Set(cacheKey, cfg.ReadTTL(), resp); err != nil {
					fmt.Fprintf(os.Stderr, "warning: cache write failed: %v\n", err)
				}
			}
			return printOutput(resp)
		},
	}
	c.Flags().BoolVar(&jsFlag, "js", false, "force browser backend (headless Chrome)")
	return c
}

func printOutput(v any) error {
	rendered, err := output.Render(formatFlag, v, cfg.Output.MarkdownMaxLength)
	if err != nil {
		return err
	}
	fmt.Println(rendered)
	return nil
}
