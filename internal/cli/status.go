package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

// StatusCmd creates the status command
func StatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the current re_gent repository status",
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

			fmt.Printf("%s %s %s\n", style.Brand("re_gent"), style.Label("repository:"), regentDir)
			fmt.Printf("%s %d\n\n", style.Label("Sessions:"), len(sessions))

			for _, sess := range sessions {
				fmt.Printf("%s %s\n", style.Label("Session:"), sess.ID)
				fmt.Printf("  %s %s\n", style.Label("Origin:"), sess.Origin)
				fmt.Printf("  %s %s\n", style.Label("Started:"), style.Timestamp(sess.StartedAt.Format("2006-01-02 15:04:05")))
				fmt.Printf("  %s %s\n", style.Label("Last seen:"), style.Timestamp(sess.LastSeenAt.Format("2006-01-02 15:04:05")))
				if sess.HeadStepID != "" {
					fmt.Printf("  %s %s\n", style.Label("Head:"), style.Hash(string(sess.HeadStepID[:8])))
				}
				fmt.Println()
			}

			// Check consistency between refs and database
			fmt.Println(style.Label("Consistency:"))
			_ = validateConsistency(s, idx) // Prints its own error messages

			return nil
		},
	}
}

// validateConsistency checks that refs and database are in sync
func validateConsistency(s *store.Store, idx *index.DB) error {
	// Get all session refs
	refFiles, err := filepath.Glob(filepath.Join(s.Root, "refs/sessions/*"))
	if err != nil {
		return err
	}

	issues := []string{}

	for _, refFile := range refFiles {
		sessionID := filepath.Base(refFile)

		// Read ref
		refHash, err := s.ReadRef("sessions/" + sessionID)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Session %s: cannot read ref: %v", sessionID, err))
			continue
		}

		// Read DB head
		dbHash, err := idx.SessionHead(sessionID)
		if err != nil {
			issues = append(issues, fmt.Sprintf("Session %s: not in database", sessionID))
			continue
		}

		// Compare
		if refHash != dbHash {
			issues = append(issues, fmt.Sprintf("Session %s: ref=%s but db=%s",
				sessionID, refHash[:8], dbHash[:8]))
		}
	}

	if len(issues) > 0 {
		fmt.Printf("  %s\n", style.Warning("⚠ Consistency issues detected:"))
		for _, issue := range issues {
			fmt.Printf("    • %s\n", issue)
		}
		fmt.Println()
		fmt.Println("  Run 'rgt reindex' to rebuild the index from refs.")
		return fmt.Errorf("consistency check failed")
	}

	fmt.Printf("  %s\n", style.Success("✓ All session refs match database"))
	return nil
}
