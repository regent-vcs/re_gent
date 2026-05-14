package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

func ShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "show <step-hash>",
		Short:        "Display a step with full context",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			s, err := store.Open(filepath.Join(cwd, ".regent"))
			if err != nil {
				return err
			}

			idx, err := index.Open(s)
			if err != nil {
				return err
			}
			defer func() { _ = idx.Close() }()

			fullHash, err := idx.NormalizeStepHash(args[0])
			if err != nil {
				return fmt.Errorf("resolve hash %s: %w", args[0], err)
			}

			step, err := s.ReadStep(fullHash)
			if err != nil {
				return fmt.Errorf("read step: %w", err)
			}

			printStepMetadata(fullHash, step)
			printStepCauses(s, step)
			if err := printStepConversation(s, idx, fullHash, step); err != nil {
				return err
			}

			return nil
		},
	}

	return cmd
}

func printStepMetadata(stepHash store.Hash, step *store.Step) {
	ts := time.Unix(0, step.TimestampNanos)
	fmt.Printf("%s %s\n", style.Label("Step:"), style.Hash(string(stepHash[:16])))
	fmt.Printf("%s %s\n", style.Label("Time:"), style.Timestamp(ts.Format("2006-01-02 15:04:05")))
	fmt.Printf("%s %s\n", style.Label("Session:"), step.SessionID)
	if step.Origin != "" {
		fmt.Printf("%s %s\n", style.Label("Origin:"), step.Origin)
	}
	if step.TurnID != "" {
		fmt.Printf("%s %s\n", style.Label("Turn:"), step.TurnID)
	}
	if step.Parent != "" {
		fmt.Printf("%s %s\n", style.Label("Parent:"), style.Hash(string(step.Parent[:16])))
	}
	fmt.Println()
}

func printStepCauses(s *store.Store, step *store.Step) {
	causes := step.Causes
	if len(causes) == 0 && step.Cause.ToolName != "" {
		causes = []store.Cause{step.Cause}
	}

	for i, cause := range causes {
		title := "Tool"
		if len(causes) > 1 {
			title = fmt.Sprintf("Tool %d", i+1)
		}
		fmt.Println(style.SectionDivider(title))
		fmt.Printf("%s %s\n", style.Label("Name:"), cause.ToolName)
		fmt.Printf("%s %s\n", style.Label("Tool Use ID:"), cause.ToolUseID)
		fmt.Println()

		fmt.Println(style.SectionDivider("Arguments"))
		printBlob(s, cause.ArgsBlob)
		fmt.Println()

		fmt.Println(style.SectionDivider("Result"))
		printBlob(s, cause.ResultBlob)
		fmt.Println()
	}
}

func printStepConversation(s *store.Store, idx *index.DB, stepHash store.Hash, step *store.Step) error {
	messages, err := idx.GetMessagesForStep(stepHash)
	if err != nil {
		return fmt.Errorf("read messages: %w", err)
	}
	if len(messages) > 0 {
		fmt.Println(style.SectionDivider("Conversation"))
		for _, msg := range messages {
			printIndexedMessage(s, msg)
		}
		return nil
	}

	if step.Transcript == "" {
		fmt.Println(style.DimText("(no conversation recorded)"))
		return nil
	}

	fmt.Println(style.SectionDivider("Conversation"))
	transcriptMessages, err := s.ReconstructTranscript(step.Transcript)
	if err != nil {
		fmt.Printf("%s\n", style.DimText(fmt.Sprintf("(error reading transcript: %v)", err)))
		return nil
	}
	for i, msg := range transcriptMessages {
		fmt.Printf("%s\n", style.DimText(fmt.Sprintf("--- Message %d ---", i+1)))
		printRawJSON(msg)
	}
	return nil
}

func printIndexedMessage(s *store.Store, msg index.Message) {
	switch msg.MessageType {
	case "user":
		fmt.Printf("%s\n%s\n\n", style.Label("User:"), msg.ContentText)
	case "assistant":
		fmt.Printf("%s\n%s\n\n", style.Label("Assistant:"), msg.ContentText)
	case "tool_call":
		fmt.Printf("%s %s %s\n", style.Label("Tool call:"), msg.ToolName, msg.ToolUseID)
		printBlob(s, store.Hash(msg.ToolInput))
		fmt.Println()
	case "tool_result":
		fmt.Printf("%s %s %s\n", style.Label("Tool result:"), msg.ToolName, msg.ToolUseID)
		printBlob(s, store.Hash(msg.ToolOutput))
		fmt.Println()
	}
}

func printBlob(s *store.Store, hash store.Hash) {
	if hash == "" {
		fmt.Println(style.DimText("(none)"))
		return
	}

	data, err := s.ReadBlob(hash)
	if err != nil {
		fmt.Println(style.DimText(fmt.Sprintf("(error reading blob: %v)", err)))
		return
	}
	printRawJSON(data)
}

func printRawJSON(data []byte) {
	var pretty interface{}
	if json.Unmarshal(data, &pretty) == nil {
		output, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println(string(output))
		return
	}
	fmt.Println(string(data))
}
