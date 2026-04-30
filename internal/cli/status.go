package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/spf13/cobra"
)

// StatusCmd creates the status command
func StatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current regent repository status",
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

			// List all sessions
			sessions, err := idx.ListAllSessions()
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions recorded yet.")
				return nil
			}

			fmt.Printf("Regent repository: %s\n", regentDir)
			fmt.Printf("Sessions: %d\n\n", len(sessions))

			for _, sess := range sessions {
				fmt.Printf("Session: %s\n", sess.ID)
				fmt.Printf("  Origin: %s\n", sess.Origin)
				fmt.Printf("  Started: %s\n", sess.StartedAt.Format("2006-01-02 15:04:05"))
				fmt.Printf("  Last seen: %s\n", sess.LastSeenAt.Format("2006-01-02 15:04:05"))
				if sess.HeadStepID != "" {
					fmt.Printf("  Head: %s\n", sess.HeadStepID[:8])
				}
				fmt.Println()
			}

			return nil
		},
	}
}
