package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

// BlameCmd creates the blame command
func BlameCmd() *cobra.Command {
	var sessionID string

	cmd := &cobra.Command{
		Use:          "blame <path>[:<line>]",
		Short:        "Show per-line provenance for a file",
		Long:         "Display which step last modified each line of a file",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Parse path and optional line number
			parts := strings.SplitN(args[0], ":", 2)
			path := parts[0]
			var line int
			if len(parts) == 2 {
				var err error
				line, err = strconv.Atoi(parts[1])
				if err != nil {
					return fmt.Errorf("invalid line number: %s", parts[1])
				}
			}

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

			// Determine session
			if sessionID == "" {
				// Use current/most recent session
				sessions, err := idx.ListAllSessions()
				if err != nil || len(sessions) == 0 {
					return fmt.Errorf("no sessions found")
				}
				sessionID = sessions[0].ID
			}

			// Get head step for session
			headStepHash, err := idx.SessionHead(sessionID)
			if err != nil {
				return err
			}

			headStep, err := s.ReadStep(headStepHash)
			if err != nil {
				return err
			}

			headTree, err := s.ReadTree(headStep.Tree)
			if err != nil {
				return err
			}

			// Find file in tree
			var entry *store.TreeEntry
			normalizedPath := filepath.ToSlash(path)
			for i := range headTree.Entries {
				if headTree.Entries[i].Path == normalizedPath {
					entry = &headTree.Entries[i]
					break
				}
			}

			if entry == nil {
				return fmt.Errorf("file not found in head tree: %s", path)
			}

			// Read blame map from separate storage
			blameMap, err := s.ReadBlameForFile(headStepHash, normalizedPath)
			if err != nil {
				return fmt.Errorf("no blame map for %s (file added before blame tracking): %w", path, err)
			}

			// Check if file is empty
			if len(blameMap.Lines) == 0 {
				fmt.Printf("(empty file - no lines to blame)\n")
				return nil
			}

			// Read file content for line display
			content, err := s.ReadBlob(entry.Blob)
			if err != nil {
				return err
			}
			lines := strings.Split(string(content), "\n")

			// Display blame
			if line > 0 {
				// Single line
				if line > len(blameMap.Lines) {
					return fmt.Errorf("line %d out of range (file has %d lines)", line, len(blameMap.Lines))
				}
				return displayBlameLine(s, blameMap.Lines[line-1], line, lines[line-1])
			}

			// Whole file
			for i, stepHash := range blameMap.Lines {
				if i < len(lines) {
					if err := displayBlameLine(s, stepHash, i+1, lines[i]); err != nil {
						return err
					}
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&sessionID, "session", "", "Session ID (defaults to most recent)")

	return cmd
}

func displayBlameLine(s *store.Store, stepHash store.Hash, lineNum int, lineContent string) error {
	step, err := s.ReadStep(stepHash)
	if err != nil {
		// Step not found - might be from Phase 2 (before blame was working)
		// or an intermediate hash from blame computation
		fmt.Printf("%-8s %-19s %-10s %s %4d %s %s\n",
			style.Hash(string(stepHash[:8])),
			style.DimText("(pre-blame)"),
			style.DimText("(unknown)"),
			style.DimText("│"),
			lineNum,
			style.DimText("│"),
			lineContent,
		)
		return nil
	}

	ts := time.Unix(0, step.TimestampNanos)

	// Format: <short hash> <timestamp> <tool name> │ <line num> │ <content>
	fmt.Printf("%-8s %s %-10s %s %4d %s %s\n",
		style.Hash(string(stepHash[:8])),
		style.Timestamp(ts.Format("2006-01-02 15:04:05")),
		step.Cause.ToolName,
		style.DimText("│"),
		lineNum,
		style.DimText("│"),
		lineContent,
	)

	return nil
}
