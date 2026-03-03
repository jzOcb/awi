package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/jzOcb/awi/internal/backend"
	"github.com/spf13/cobra"
)

func newInteractCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "interact <url>",
		Short: "Open a page and interact with it via agent-browser commands",
		Long: `Opens a URL in a headless browser and enters an interactive REPL.
Available commands in the REPL:
  snapshot          - Get accessibility tree with refs
  click <sel>       - Click an element
  fill <sel> <text> - Fill an input field
  type <sel> <text> - Type into an element
  get text <sel>    - Get text content
  get html <sel>    - Get innerHTML
  screenshot [path] - Take a screenshot
  eval <js>         - Run JavaScript
  close / quit      - Close browser and exit`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			bb := backend.NewBrowserBackend(timeoutFlag)
			if !bb.Available() {
				return fmt.Errorf("agent-browser not found; install with: npm install -g agent-browser")
			}

			ctx, cancel := commandContext()
			defer cancel()

			// Open the page
			fmt.Fprintf(os.Stderr, "Opening %s ...\n", targetURL)
			snap, err := bb.Snapshot(ctx, targetURL)
			if err != nil {
				return fmt.Errorf("failed to open page: %w", err)
			}
			fmt.Println(snap)
			fmt.Fprintln(os.Stderr, "\nReady. Type commands (snapshot, click, fill, get, screenshot, eval, close):")

			scanner := bufio.NewScanner(os.Stdin)
			fmt.Fprint(os.Stderr, "awi> ")
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					fmt.Fprint(os.Stderr, "awi> ")
					continue
				}
				if line == "close" || line == "quit" || line == "exit" {
					_ = bb.Close()
					fmt.Fprintln(os.Stderr, "Browser closed.")
					return nil
				}

				parts := strings.Fields(line)
				out, err := bb.Act(ctx, parts...)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
				} else if out != "" {
					fmt.Println(out)
				}
				fmt.Fprint(os.Stderr, "awi> ")
			}

			_ = bb.Close()
			return scanner.Err()
		},
	}
	return c
}
