package cli

import (
	"fmt"

	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

// These are set at build time via -ldflags. See .goreleaser.yaml and the
// Makefile for the exact symbol paths used by the linker.
var (
	// Version is the release version (e.g. "1.1.0"). Defaults to "dev" for
	// local/unstamped builds.
	Version = "dev"
	// Commit is the git commit the binary was built from.
	Commit = "unknown"
	// Date is the build timestamp (RFC3339). Empty for unstamped builds.
	Date = ""
)

// VersionString returns the human-readable version line shared by the
// `version` subcommand and the root `--version` flag.
func VersionString() string {
	s := fmt.Sprintf("%s version %s (commit: %s)", style.Brand("re_gent"), Version, Commit)
	if Date != "" {
		s += fmt.Sprintf(" built %s", Date)
	}
	return s
}

// VersionCmd creates the version command
func VersionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(VersionString())
		},
	}

	return cmd
}
