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

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show step history",
		Long:  "Display steps in reverse-chronological order with tool names and causes.",
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

			fmt.Printf("Session: %s (%d steps)\n\n", sessionID, len(steps))

			for _, step := range steps {
				fmt.Printf("%s  %s  %s\n",
					step.Hash[:8],
					step.Timestamp.Format("2006-01-02 15:04:05"),
					step.ToolName,
				)
				if step.ToolUseID != "" {
					fmt.Printf("    tool_use_id: %s\n", step.ToolUseID)
				}
				if step.ParentHash != "" {
					fmt.Printf("    parent: %s\n", step.ParentHash[:8])
				}
				fmt.Println()
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID to show (defaults to most recent)")
	cmd.Flags().IntVarP(&limit, "limit", "n", 20, "Maximum number of steps to show")

	return cmd
}
