package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/regent-vcs/regent/internal/store"
	"github.com/spf13/cobra"
)

// CatCmd creates the cat command for dumping objects
func CatCmd() *cobra.Command {
	var pretty bool

	cmd := &cobra.Command{
		Use:   "cat <hash>",
		Short: "Dump an object by hash",
		Long:  "Debug command: reads and displays the content of any object in the store.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hash := store.Hash(args[0])

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}

			regentDir := filepath.Join(cwd, ".regent")
			s, err := store.Open(regentDir)
			if err != nil {
				return err
			}

			data, err := s.ReadBlob(hash)
			if err != nil {
				return err
			}

			// Try to detect and pretty-print JSON
			if pretty && json.Valid(data) {
				var obj interface{}
				if err := json.Unmarshal(data, &obj); err == nil {
					prettyData, err := json.MarshalIndent(obj, "", "  ")
					if err == nil {
						fmt.Println(string(prettyData))
						return nil
					}
				}
			}

			// Fall back to raw output
			fmt.Print(string(data))
			return nil
		},
	}

	cmd.Flags().BoolVarP(&pretty, "pretty", "p", true, "Pretty-print JSON objects")

	return cmd
}
