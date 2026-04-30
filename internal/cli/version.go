package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is set at build time via -ldflags
	Version = "dev"
	// Commit is set at build time via -ldflags
	Commit = "unknown"
)

// VersionCmd creates the version command
func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("rgt version %s (commit: %s)\n", Version, Commit)
		},
	}

	return cmd
}
