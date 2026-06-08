package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

const (
	sessionsFormatText = "text"
	sessionsFormatJSON = "json"
)

type sessionsJSONOutput struct {
	TotalSessions int           `json:"total_sessions"`
	Sessions      []sessionJSON `json:"sessions"`
}

type sessionJSON struct {
	SessionID    string `json:"session_id"`
	StepCount    int    `json:"step_count"`
	LastActivity string `json:"last_activity"`
	AgentID      string `json:"agent_id"`
}

type sessionJSONStats struct {
	StepCount int
	AgentID   string
}

// SessionsCmd creates the sessions command
func SessionsCmd() *cobra.Command {
	var outputFormat string

	cmd := &cobra.Command{
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

			w := cmd.OutOrStdout()

			if outputFormat == sessionsFormatJSON {
				return writeSessionsJSON(w, s, idx, sessions)
			}

			return writeSessionsText(w, sessions)
		},
	}

	cmd.Flags().StringVar(&outputFormat, "format", sessionsFormatText, "Output format: text or json")

	return cmd
}

func writeSessionsText(w io.Writer, sessions []index.SessionInfo) error {
	if len(sessions) == 0 {
		fmt.Fprintln(w, "No sessions recorded yet.")
		return nil
	}

	fmt.Fprintf(w, "%s %d\n\n", style.Label("Total sessions:"), len(sessions))

	for _, sess := range sessions {
		fmt.Fprintf(w, "%s %s\n", style.Label("Session:"), sess.ID)
		fmt.Fprintf(w, "  %s     %s\n", style.Label("Origin:"), sess.Origin)
		if sess.Model != "" {
			fmt.Fprintf(w, "  %s      %s\n", style.Label("Model:"), sess.Model)
		}
		if sess.PermissionMode != "" {
			fmt.Fprintf(w, "  %s %s\n", style.Label("Permission:"), sess.PermissionMode)
		}
		fmt.Fprintf(w, "  %s    %s\n", style.Label("Started:"), style.Timestamp(sess.StartedAt.Format("2006-01-02 15:04:05")))
		fmt.Fprintf(w, "  %s  %s\n", style.Label("Last seen:"), style.Timestamp(sess.LastSeenAt.Format("2006-01-02 15:04:05")))

		if sess.ForkedFromSession != "" {
			fmt.Fprintf(w, "  %s     Forked from session %s at step %s\n",
				style.Label("Fork:"),
				style.Hash(sess.ForkedFromSession),
				style.Hash(string(sess.ForkedFromStep[:8])))
			if sess.ForkDetectedAt != nil {
				fmt.Fprintf(w, "             %s\n", style.Timestamp(sess.ForkDetectedAt.Format("2006-01-02 15:04:05")))
			}
		}

		if sess.HeadStepID != "" {
			fmt.Fprintf(w, "  %s       %s\n", style.Label("Head:"), style.Hash(string(sess.HeadStepID[:16])))
		}
		fmt.Fprintln(w)
	}

	return nil
}

func writeSessionsJSON(w io.Writer, s *store.Store, idx *index.DB, sessions []index.SessionInfo) error {
	output := sessionsJSONOutput{
		TotalSessions: len(sessions),
		Sessions:      make([]sessionJSON, 0, len(sessions)),
	}

	for _, sess := range sessions {
		stats, err := loadSessionJSONStats(s, idx, sess)
		if err != nil {
			return err
		}

		output.Sessions = append(output.Sessions, sessionJSON{
			SessionID:    sess.ID,
			StepCount:    stats.StepCount,
			LastActivity: sess.LastSeenAt.UTC().Format(time.RFC3339),
			AgentID:      stats.AgentID,
		})
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func loadSessionJSONStats(s *store.Store, idx *index.DB, sess index.SessionInfo) (sessionJSONStats, error) {
	count, err := idx.CountSteps(sess.ID)
	if err != nil {
		return sessionJSONStats{}, fmt.Errorf("count steps for session %s: %w", sess.ID, err)
	}

	agentID := sess.Origin
	if sess.HeadStepID != "" {
		headStep, err := s.ReadStep(sess.HeadStepID)
		if err != nil {
			return sessionJSONStats{}, fmt.Errorf("read head step for session %s: %w", sess.ID, err)
		}
		if headStep.AgentID != "" {
			agentID = headStep.AgentID
		}
	}

	return sessionJSONStats{
		StepCount: count,
		AgentID:   agentID,
	}, nil
}
