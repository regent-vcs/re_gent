package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/spf13/cobra"
)

// LogCmd creates the log command
func LogCmd() *cobra.Command {
	var sessionID string
	var limit int
	var oneline bool
	var jsonOut bool
	var stat bool

	cmd := &cobra.Command{
		Use:          "log",
		Short:        "Show step history",
		Long:         "Display steps in reverse-chronological order with tool names and files affected.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			regentDir := filepath.Join(cwd, ".regent")
			s, err := store.Open(regentDir)
			if err != nil {
				return err
			}

			idx, err := index.Open(s)
			if err != nil {
				return err
			}
			defer func() { _ = idx.Close() }()

			// If no session specified, use the most recent one
			if sessionID == "" {
				sessions, err := idx.ListAllSessions()
				if err != nil {
					return err
				}
				if len(sessions) == 0 {
					fmt.Println("No sessions found.")
					return nil
				}
				sessionID = sessions[0].ID
			}

			steps, err := idx.ListSteps(sessionID, limit)
			if err != nil {
				return err
			}

			if len(steps) == 0 {
				fmt.Printf("No steps found for session %s\n", sessionID)
				return nil
			}

			// Enrich steps with files, args, results
			enriched, err := enrichSteps(s, steps)
			if err != nil {
				return fmt.Errorf("enrich steps: %w", err)
			}

			// Determine formatter
			var formatter LogFormatter
			if oneline {
				formatter = &OnelineFormatter{}
			} else if jsonOut {
				formatter = &JSONFormatter{}
			} else if stat {
				formatter = &StatFormatter{}
			} else {
				formatter = &DefaultFormatter{}
			}

			// Format and output
			return formatter.Format(enriched, sessionID, os.Stdout)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to show (defaults to most recent)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of steps to show")
	cmd.Flags().BoolVar(&oneline, "oneline", false, "Show one line per step (compact)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&stat, "stat", false, "Show file statistics")

	return cmd
}
