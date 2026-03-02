package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootSmokeFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		contains []string
	}{
		{
			name:     "help",
			args:     []string{"--help"},
			contains: []string{"AWI - Agentic Web Interface", "Usage:"},
		},
		{
			name:     "version",
			args:     []string{"--version"},
			contains: []string{"awi version", Version},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg = nil
			router = nil
			diskCache = nil
			showVersionFlag = false

			_ = rootCmd.Flags().Set("help", "false")
			_ = rootCmd.PersistentFlags().Set("version", "false")

			buf := &bytes.Buffer{}
			rootCmd.SetOut(buf)
			rootCmd.SetErr(buf)
			rootCmd.SetArgs(tt.args)

			if _, err := rootCmd.ExecuteC(); err != nil {
				t.Fatalf("ExecuteC() error = %v", err)
			}

			out := buf.String()
			for _, needle := range tt.contains {
				if !strings.Contains(out, needle) {
					t.Fatalf("output %q does not contain %q", out, needle)
				}
			}
		})
	}
}
