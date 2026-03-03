package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/jzOcb/awi/internal/backend"
	"github.com/spf13/cobra"
)

func newActCmd() *cobra.Command {
	var (
		screenshotPath string
		keepOpen       bool
	)

	c := &cobra.Command{
		Use:   "act <url> [command] [args...]",
		Short: "Execute browser actions for AI agents",
		Long: `Opens a URL and executes agent-browser commands.
Without a command, returns the accessibility snapshot (ideal for AI agents).

Examples:
  awi act https://example.com                     # Get snapshot
  awi act https://example.com click @e5            # Click ref
  awi act https://example.com fill @e3 "hello"     # Fill input
  awi act https://example.com screenshot out.png   # Screenshot
  awi act https://example.com snapshot             # Explicit snapshot`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			bb := backend.NewBrowserBackend(timeoutFlag)
			if !bb.Available() {
				return fmt.Errorf("agent-browser not found; install with: npm install -g agent-browser")
			}

			ctx, cancel := commandContext()
			defer cancel()

			// Open page and get snapshot
			fmt.Fprintf(os.Stderr, "Opening %s ...\n", targetURL)
			snap, err := bb.Snapshot(ctx, targetURL)
			if err != nil {
				return fmt.Errorf("failed to open page: %w", err)
			}

			// No action args: just output snapshot
			if len(args) == 1 {
				fmt.Println(snap)
				if !keepOpen {
					_ = bb.Close()
				}
				return nil
			}

			// Execute the action
			action := args[1:]
			out, err := bb.Act(ctx, action...)
			if err != nil {
				_ = bb.Close()
				return fmt.Errorf("action %q failed: %w", strings.Join(action, " "), err)
			}
			if out != "" {
				fmt.Println(out)
			}

			// Take screenshot if requested
			if screenshotPath != "" {
				_, sErr := bb.Act(ctx, "screenshot", screenshotPath)
				if sErr != nil {
					fmt.Fprintf(os.Stderr, "screenshot error: %v\n", sErr)
				} else {
					fmt.Fprintf(os.Stderr, "Screenshot saved to %s\n", screenshotPath)
				}
			}

			// Get updated snapshot after action
			newSnap, err := bb.Act(ctx, "snapshot")
			if err == nil && newSnap != "" {
				fmt.Fprintln(os.Stderr, "\n--- Updated Snapshot ---")
				fmt.Println(newSnap)
			}

			if !keepOpen {
				_ = bb.Close()
			}
			return nil
		},
	}
	c.Flags().StringVar(&screenshotPath, "screenshot", "", "take screenshot after action")
	c.Flags().BoolVar(&keepOpen, "keep-open", false, "keep browser open after action")
	return c
}
