package cmd

import (
	"fmt"

	"github.com/jzOcb/awi/internal/backend"
	"github.com/spf13/cobra"
)

func newActCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "act <ref> <action> [text]",
		Short: "Run agent-browser action for a snapshot ref",
		Long: `Runs a browser action against a ref from 'awi interact' snapshot.

Examples:
  awi act @e5 click
  awi act @e3 fill "hello@example.com"
  awi act @e3 type "search query"`,
		Args: cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			action := args[1]
			text := ""
			if len(args) == 3 {
				text = args[2]
			}

			bb := backend.NewBrowserBackend(timeoutFlag)
			if !bb.Available() {
				return fmt.Errorf("agent-browser not found; install with: npm install -g agent-browser")
			}

			ctx, cancel := commandContext()
			defer cancel()

			cmdArgs := []string{action, ref}
			if text != "" {
				cmdArgs = append(cmdArgs, text)
			}

			out, err := bb.Act(ctx, cmdArgs...)
			if err != nil {
				return fmt.Errorf("action failed (%s %s): %w", action, ref, err)
			}
			if out != "" {
				fmt.Println(out)
			}
			return nil
		},
	}
	return c
}
