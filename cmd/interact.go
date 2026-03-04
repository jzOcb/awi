package cmd

import (
	"fmt"
	"os"

	"github.com/jzOcb/awi/internal/backend"
	"github.com/spf13/cobra"
)

func newInteractCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "interact <url>",
		Short: "Open URL and return agent-browser snapshot with refs",
		Long: `Opens the URL via agent-browser and returns an accessibility snapshot.
Use refs from this snapshot (e.g. @e1) with 'awi act'.`,
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
			return nil
		},
	}
	return c
}
