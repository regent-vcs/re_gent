package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/regent-vcs/regent/internal/adapter/codex"
	"github.com/spf13/cobra"
)

// CodexCmd creates the Codex adapter command group.
func CodexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Import or watch Codex Desktop session history",
		Long:  "Read local Codex rollout JSONL files and import them into the current .regent repository.",
	}

	cmd.AddCommand(codexImportCmd())
	cmd.AddCommand(codexWatchCmd())
	return cmd
}

func codexImportCmd() *cobra.Command {
	var projectRoot string
	var codexHome string
	var changesOnly bool

	cmd := &cobra.Command{
		Use:          "import",
		Short:        "Import Codex history for one project",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectRoot == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				projectRoot = cwd
			}

			absRoot, err := filepath.Abs(projectRoot)
			if err != nil {
				return fmt.Errorf("resolve project root: %w", err)
			}

			return codex.RunImport(codex.Options{
				ProjectRoot: absRoot,
				CodexHome:   codexHome,
				ChangesOnly: changesOnly,
				WatchMode:   false,
			})
		},
	}

	cmd.Flags().StringVar(&projectRoot, "project", "", "Absolute or relative project path whose .regent store should receive Codex history")
	cmd.Flags().StringVar(&codexHome, "codex-home", "", "Path to Codex home (defaults to ~/.codex)")
	cmd.Flags().BoolVar(&changesOnly, "changes-only", true, "Only record turns that change the workspace tree")

	return cmd
}

func codexWatchCmd() *cobra.Command {
	var projectRoot string
	var codexHome string
	var poll time.Duration
	var changesOnly bool

	cmd := &cobra.Command{
		Use:          "watch",
		Short:        "Continuously watch Codex history for one project",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if projectRoot == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				projectRoot = cwd
			}

			absRoot, err := filepath.Abs(projectRoot)
			if err != nil {
				return fmt.Errorf("resolve project root: %w", err)
			}

			return codex.RunWatch(codex.Options{
				ProjectRoot:  absRoot,
				CodexHome:    codexHome,
				PollInterval: poll,
				ChangesOnly:  changesOnly,
				WatchMode:    true,
			})
		},
	}

	cmd.Flags().StringVar(&projectRoot, "project", "", "Absolute or relative project path whose .regent store should receive Codex history")
	cmd.Flags().StringVar(&codexHome, "codex-home", "", "Path to Codex home (defaults to ~/.codex)")
	cmd.Flags().DurationVar(&poll, "poll", 2*time.Second, "Polling interval for scanning Codex rollout JSONL files")
	cmd.Flags().BoolVar(&changesOnly, "changes-only", true, "Only record turns that change the workspace tree")

	return cmd
}
