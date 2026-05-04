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
	var conversation bool
	var files bool
	var conversationOnly bool
	var filesOnly bool
	var graph bool

	cmd := &cobra.Command{
		Use:          "log [session-id]",
		Short:        "Show step history",
		Long:         "Display steps in reverse-chronological order with tool names and files affected.",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
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

			// Parse session ID from args or flag or auto-detect
			if len(args) > 0 {
				sessionID = args[0]
			}

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

			// Handle filter flags
			if conversationOnly && filesOnly {
				return fmt.Errorf("--conversation-only and --files-only are mutually exclusive")
			}

			// Set defaults: show both conversation and files
			if !cmd.Flags().Changed("conversation") && !cmd.Flags().Changed("files") {
				conversation = true
				files = true
			}

			// Apply filters
			if conversationOnly {
				conversation = true
				files = false
			}
			if filesOnly {
				conversation = false
				files = true
			}

			steps, err := idx.ListSteps(sessionID, limit)
			if err != nil {
				return err
			}

			if len(steps) == 0 {
				fmt.Printf("No steps found for session %s\n", sessionID)
				return nil
			}

			// Enrich steps with files, args, results, and optionally file diffs and graph
			enriched, err := enrichSteps(s, steps, files, graph)
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
			return formatter.Format(enriched, sessionID, conversation, files, os.Stdout)
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to show (defaults to most recent)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of steps to show")
	cmd.Flags().BoolVar(&oneline, "oneline", false, "Show one line per step (compact)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	cmd.Flags().BoolVar(&stat, "stat", false, "Show file statistics")
	cmd.Flags().BoolVar(&conversation, "conversation", false, "Show conversation transcript (deprecated, now default)")
	cmd.Flags().BoolVar(&files, "files", false, "Show file change statistics (deprecated, now default)")
	cmd.Flags().BoolVar(&conversationOnly, "conversation-only", false, "Show only conversation (hide files)")
	cmd.Flags().BoolVar(&filesOnly, "files-only", false, "Show only files (hide conversation)")
	cmd.Flags().BoolVar(&graph, "graph", false, "Show step lineage as ASCII graph")

	return cmd
}
