package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

// ShowCmd creates the show command
func ShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "show <step-hash>",
		Short:        "Display a step with full context (tool call + conversation)",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			stepHashPrefix := args[0]

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			regentDir := filepath.Join(cwd, ".regent")
			s, err := store.Open(regentDir)
			if err != nil {
				return err
			}

			// Resolve short hash to full hash
			fullHash, err := s.ResolveShortHash(stepHashPrefix)
			if err != nil {
				return fmt.Errorf("resolve hash %s: %w", stepHashPrefix, err)
			}

			// Read step
			step, err := s.ReadStep(fullHash)
			if err != nil {
				return fmt.Errorf("read step: %w", err)
			}

			// Display step metadata
			ts := time.Unix(0, step.TimestampNanos)
			fmt.Printf("%s %s\n", style.Label("Step:"), style.Hash(string(fullHash[:16])))
			fmt.Printf("%s %s\n", style.Label("Time:"), style.Timestamp(ts.Format("2006-01-02 15:04:05")))
			fmt.Printf("%s %s\n", style.Label("Tool:"), step.Cause.ToolName)
			fmt.Printf("%s %s\n", style.Label("Tool Use ID:"), step.Cause.ToolUseID)
			if step.Parent != "" {
				fmt.Printf("%s %s\n", style.Label("Parent:"), style.Hash(string(step.Parent[:16])))
			}
			fmt.Println()

			// Display tool args
			fmt.Println(style.SectionDivider("Tool Arguments"))
			argsBlob, _ := s.ReadBlob(step.Cause.ArgsBlob)
			var argsPretty interface{}
			if json.Unmarshal(argsBlob, &argsPretty) == nil {
				pretty, _ := json.MarshalIndent(argsPretty, "", "  ")
				fmt.Println(string(pretty))
			} else {
				fmt.Println(string(argsBlob))
			}
			fmt.Println()

			// Display tool result
			fmt.Println(style.SectionDivider("Tool Result"))
			resultBlob, _ := s.ReadBlob(step.Cause.ResultBlob)
			var resultPretty interface{}
			if json.Unmarshal(resultBlob, &resultPretty) == nil {
				pretty, _ := json.MarshalIndent(resultPretty, "", "  ")
				fmt.Println(string(pretty))
			} else {
				fmt.Println(string(resultBlob))
			}
			fmt.Println()

			// Display conversation (if available)
			if step.Transcript != "" {
				fmt.Println(style.SectionDivider("Conversation"))
				messages, err := s.ReconstructTranscript(step.Transcript)
				if err != nil {
					fmt.Printf("%s\n", style.DimText(fmt.Sprintf("(error reading transcript: %v)", err)))
				} else {
					for i, msg := range messages {
						var msgPretty interface{}
						if json.Unmarshal(msg, &msgPretty) == nil {
							pretty, _ := json.MarshalIndent(msgPretty, "", "  ")
							fmt.Printf("%s\n%s\n", style.DimText(fmt.Sprintf("─── Message %d ───", i+1)), string(pretty))
						}
					}
				}
			} else {
				fmt.Println(style.DimText("(no conversation recorded)"))
			}

			return nil
		},
	}

	return cmd
}
