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
		Short: "Regent - version control for AI agent activity",
		Long:  "Regent is a content-addressed version control system for AI agent activity.\nIt captures what an agent did, why, and lets you blame, log, and rewind across sessions.",
	}

	rootCmd.AddCommand(cli.InitCmd())
	rootCmd.AddCommand(cli.StatusCmd())
	rootCmd.AddCommand(cli.LogCmd())
	rootCmd.AddCommand(cli.SessionsCmd())
	rootCmd.AddCommand(cli.CatCmd())
	rootCmd.AddCommand(cli.HookCmd())
	rootCmd.AddCommand(cli.VersionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
