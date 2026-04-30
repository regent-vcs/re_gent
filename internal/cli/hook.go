package cli

import (
	"os"

	"github.com/regent-vcs/regent/internal/hook"
	"github.com/spf13/cobra"
)

// HookCmd creates the hook command (invoked by Claude Code PostToolUse)
func HookCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "hook",
		Short:  "Process a PostToolUse hook (internal)",
		Long:   "Internal command invoked by Claude Code after each tool use. Reads payload from stdin and creates a step.",
		Hidden: true, // Don't show in help (internal use only)
		RunE: func(cmd *cobra.Command, args []string) error {
			return hook.Run(os.Stdin, os.Stdout)
		},
	}
}
