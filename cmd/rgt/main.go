package main

import (
	"fmt"
	"os"

	"github.com/regent-vcs/regent/internal/cli"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "rgt",
		Short: "re_gent - version control for AI agent activity",
		Long:  "re_gent is a content-addressed version control system for AI agent activity.\nIt captures what an agent did, why, and lets you blame, log, and inspect steps across sessions.",
	}

	// Add commands in desired help order (init first, then common commands)
	rootCmd.AddCommand(cli.InitCmd())
	rootCmd.AddCommand(cli.LogCmd())
	rootCmd.AddCommand(cli.StatusCmd())
	rootCmd.AddCommand(cli.BlameCmd())
	rootCmd.AddCommand(cli.ShowCmd())
	rootCmd.AddCommand(cli.SessionsCmd())
	rootCmd.AddCommand(cli.HookCmd())
	rootCmd.AddCommand(MessageHookCmd())
	rootCmd.AddCommand(ToolBatchHookCmd())
	rootCmd.AddCommand(CodexHookCmd())
	rootCmd.AddCommand(cli.CatCmd())
	rootCmd.AddCommand(cli.VersionCmd())

	// Disable alphabetical sorting to preserve our order
	rootCmd.CompletionOptions.DisableDefaultCmd = false
	cobra.EnableCommandSorting = false

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
