package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/spf13/cobra"
)

// InitCmd creates the init command
func InitCmd() *cobra.Command {
	var skipHook bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new regent repository",
		Long:  "Creates a .regent directory in the current workspace and sets up the object store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			// Initialize store
			s, err := store.Init(cwd)
			if err != nil {
				return err
			}

			// Initialize index
			idx, err := index.Open(s)
			if err != nil {
				return fmt.Errorf("initialize index: %w", err)
			}
			defer func() { _ = idx.Close() }()

			fmt.Printf("Initialized regent repository in %s\n", filepath.Join(cwd, ".regent"))
			fmt.Println()

			// Offer to configure hook (unless --skip-hook)
			if !skipHook {
				if err := offerHookInstall(cwd); err != nil {
					// Non-fatal: warn but don't fail init
					fmt.Printf("⚠️  Could not configure hook: %v\n", err)
					printManualInstructions()
				}
			} else {
				printManualInstructions()
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&skipHook, "skip-hook", false, "Skip automatic hook configuration")

	return cmd
}

// offerHookInstall prompts user and configures the hook if approved
func offerHookInstall(projectRoot string) error {
	fmt.Print("Enable automatic tracking in Claude Code? [Y/n]: ")

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	// Default to yes (Y is uppercase in prompt)
	if response == "" || response == "y" || response == "yes" {
		if err := installHook(projectRoot); err != nil {
			return err
		}
		fmt.Println("✓ Configured PostToolUse hook in .claude/settings.json")
		fmt.Println()
		return nil
	}

	// User declined
	fmt.Println("Skipped hook configuration.")
	fmt.Println()
	printManualInstructions()
	return nil
}

// installHook adds the PostToolUse hook to .claude/settings.json
func installHook(projectRoot string) error {
	claudeDir := filepath.Join(projectRoot, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	// Ensure .claude directory exists
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}

	// Read existing settings or start fresh
	var settings map[string]interface{}
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		// File exists, try to parse it
		if err := json.Unmarshal(data, &settings); err != nil {
			// Invalid JSON - backup and start fresh
			backupPath := settingsPath + ".backup"
			_ = os.Rename(settingsPath, backupPath)
			settings = make(map[string]interface{})
		}
	} else {
		// File doesn't exist, start fresh
		settings = make(map[string]interface{})
	}

	// Check if hook already configured
	if hooks, ok := settings["hooks"].(map[string]interface{}); ok {
		if postToolUse, ok := hooks["PostToolUse"].(string); ok && postToolUse == "rgt hook" {
			fmt.Println("✓ PostToolUse hook already configured")
			fmt.Println()
			return nil
		}
	}

	// Add or update hooks section
	if settings["hooks"] == nil {
		settings["hooks"] = make(map[string]interface{})
	}

	hooks := settings["hooks"].(map[string]interface{})
	hooks["PostToolUse"] = "rgt hook"

	// Write back with pretty formatting
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}

	if err := os.WriteFile(settingsPath, output, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
}

// printManualInstructions shows how to configure the hook manually
func printManualInstructions() {
	fmt.Println("To enable tracking manually, add this to .claude/settings.json:")
	fmt.Println()
	fmt.Println("  {")
	fmt.Println("    \"hooks\": {")
	fmt.Println("      \"PostToolUse\": \"rgt hook\"")
	fmt.Println("    }")
	fmt.Println("  }")
	fmt.Println()
}
