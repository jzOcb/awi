package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/jasonz/webscout/internal/backend"
	"github.com/jasonz/webscout/internal/output"
	"github.com/jasonz/webscout/internal/retry"
)

type batchItem struct {
	URL      string                `json:"url"`
	Success  bool                  `json:"success"`
	Error    string                `json:"error,omitempty"`
	Response *backend.ReadResponse `json:"response,omitempty"`
}

type batchResponse struct {
	Total   int         `json:"total"`
	Success int         `json:"success"`
	Failed  int         `json:"failed"`
	Items   []batchItem `json:"items"`
}

type batchJob struct {
	index int
	url   string
}

func newBatchCmd() *cobra.Command {
	var concurrency int
	c := &cobra.Command{
		Use:   "batch <file.txt>",
		Short: "Read a batch of URLs from file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			urls, err := readLines(args[0])
			if err != nil {
				return err
			}
			if len(urls) == 0 {
				return fmt.Errorf("no URLs found in file")
			}
			if concurrency <= 0 {
				concurrency = 4
			}

			ctx, cancel := commandContext()
			defer cancel()

			out := make([]batchItem, len(urls))
			jobs := make(chan batchJob)
			var wg sync.WaitGroup

			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for job := range jobs {
						resp, err := retry.DoValue(ctx, retryPolicy, func() (*backend.ReadResponse, error) {
							return router.Read(ctx, backendFlag, &backend.ReadRequest{URL: job.url, Timeout: timeoutFlag, Proxy: proxyFlag})
						})
						if err != nil {
							out[job.index] = batchItem{URL: job.url, Success: false, Error: err.Error()}
							continue
						}
						out[job.index] = batchItem{URL: job.url, Success: true, Response: resp}
					}
				}()
			}

			for i, u := range urls {
				jobs <- batchJob{index: i, url: u}
			}
			close(jobs)
			wg.Wait()

			success := 0
			for _, item := range out {
				if item.Success {
					success++
				}
			}
			result := &batchResponse{Total: len(out), Success: success, Failed: len(out) - success, Items: out}

			rendered, err := output.Render(formatFlag, result, cfg.Output.MarkdownMaxLength)
			if err != nil {
				return err
			}
			fmt.Println(rendered)
			return nil
		},
	}
	c.Flags().IntVar(&concurrency, "concurrency", 4, "number of concurrent workers")
	return c
}

func readLines(path string) ([]string, error) {
	var r io.Reader
	if path == "-" {
		r = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		r = f
	}

	out := make([]string, 0)
	s := bufio.NewScanner(r)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func intToString(v int) string {
	return fmt.Sprintf("%d", v)
}
