package cli

import (
	"fmt"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

// SessionsCmd creates the sessions command
func SessionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List all sessions",
		Long:  "Display all recorded sessions with their metadata and head steps.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStoreFromCWD()
			if err != nil {
				return err
			}

			idx, err := index.Open(s)
			if err != nil {
				return err
			}
			defer func() { _ = idx.Close() }()

			sessions, err := idx.ListAllSessions()
			if err != nil {
				return err
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions recorded yet.")
				return nil
			}

			fmt.Printf("%s %d\n\n", style.Label("Total sessions:"), len(sessions))

			for _, sess := range sessions {
				fmt.Printf("%s %s\n", style.Label("Session:"), sess.ID)
				fmt.Printf("  %s     %s\n", style.Label("Origin:"), sess.Origin)
				if sess.Model != "" {
					fmt.Printf("  %s      %s\n", style.Label("Model:"), sess.Model)
				}
				if sess.PermissionMode != "" {
					fmt.Printf("  %s %s\n", style.Label("Permission:"), sess.PermissionMode)
				}
				fmt.Printf("  %s    %s\n", style.Label("Started:"), style.Timestamp(sess.StartedAt.Format("2006-01-02 15:04:05")))
				fmt.Printf("  %s  %s\n", style.Label("Last seen:"), style.Timestamp(sess.LastSeenAt.Format("2006-01-02 15:04:05")))

				if sess.ForkedFromSession != "" {
					fmt.Printf("  %s     Forked from session %s at step %s\n",
						style.Label("Fork:"),
						style.Hash(sess.ForkedFromSession),
						style.Hash(string(sess.ForkedFromStep[:8])))
					if sess.ForkDetectedAt != nil {
						fmt.Printf("             %s\n", style.Timestamp(sess.ForkDetectedAt.Format("2006-01-02 15:04:05")))
					}
				}

				if sess.HeadStepID != "" {
					fmt.Printf("  %s       %s\n", style.Label("Head:"), style.Hash(string(sess.HeadStepID[:16])))
				}
				fmt.Println()
			}

			return nil
		},
	}
}
