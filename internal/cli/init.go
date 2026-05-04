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
	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

// InitCmd creates the init command
func InitCmd() *cobra.Command {
	var skipHook bool

	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Initialize a new regent repository",
		Long:         "Creates a .regent directory in the current workspace and sets up the object store.",
		SilenceUsage: true, // Don't show usage on logical errors
		Annotations: map[string]string{
			"commandOrder": "0", // Ensure init appears first in help
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			// Print header
			printHeader()

			// Step 1: Initialize repository
			printStep(1, 3, "Initialize Repository")
			s, err := store.Init(cwd)
			if err != nil {
				return err
			}

			idx, err := index.Open(s)
			if err != nil {
				return fmt.Errorf("initialize index: %w", err)
			}
			defer func() { _ = idx.Close() }()

			if err := createRegentGitignore(cwd); err != nil {
				fmt.Printf("  %s Could not create .regent/.gitignore: %v\n", style.Warning(""), err)
			}

			fmt.Printf("  %s Created .regent/ directory\n", style.Success(""))
			fmt.Printf("  %s Initialized object store\n", style.Success(""))
			fmt.Printf("  %s Created SQLite index\n", style.Success(""))
			fmt.Println()

			// Step 2: Configure hook (unless --skip-hook)
			if !skipHook {
				printStep(2, 3, "Configure Claude Code Hook")
				if err := offerHookInstall(cwd); err != nil {
					fmt.Printf("  %s Could not configure hook: %v\n", style.Warning(""), err)
					printManualInstructions()
				}
			} else {
				printStep(2, 3, "Configure Claude Code Hook (skipped)")
				printManualInstructions()
			}

			// Step 3: Install Claude skills
			printStep(3, 3, "Install Claude Skills")
			if err := offerSkillInstall(cwd); err != nil {
				fmt.Printf("  %s Could not install skills: %v\n", style.Warning(""), err)
			}

			// Summary
			printSummary(cwd)

			return nil
		},
	}

	cmd.Flags().BoolVar(&skipHook, "skip-hook", false, "Skip automatic hook configuration")

	return cmd
}

// printHeader prints the init wizard header
func printHeader() {
	fmt.Println()
	fmt.Println(style.DividerFull(""))
	fmt.Printf("  %s - Version Control for AI Agent Activity\n", style.Brand("re_gent"))
	fmt.Println(style.DividerFull(""))
	fmt.Println()
}

// printStep prints a step header
func printStep(current, total int, title string) {
	fmt.Println(style.SectionHeader(fmt.Sprintf("Step %d/%d: %s", current, total, title)))
	fmt.Println()
}

// printSummary prints the completion summary
func printSummary(projectRoot string) {
	fmt.Println()
	fmt.Println(style.DividerFull(""))
	fmt.Printf("  %s Initialization Complete!\n", style.Success(""))
	fmt.Println(style.DividerFull(""))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  • Start a Claude Code session in this directory")
	fmt.Println("  • Make some changes (the hook will capture them)")
	fmt.Println("  • Run: rgt log")
	fmt.Println("  • Run: rgt blame <file>")
	fmt.Println("  • Use: /log, /blame, /show, /rewind in Claude")
	fmt.Println()
	fmt.Printf("%s %s\n", style.Label("Repository:"), filepath.Join(projectRoot, ".regent"))
	fmt.Println()
}

// offerHookInstall prompts user and configures the hook if approved
func offerHookInstall(projectRoot string) error {
	fmt.Printf("%s captures step history automatically via Claude Code hooks.\n", style.Brand("re_gent"))
	fmt.Println()
	fmt.Println("This will configure .claude/settings.json to run 'rgt hook'")
	fmt.Println("after each tool use (Write, Edit, Bash, etc.).")
	fmt.Println()
	fmt.Print(style.Prompt("Enable automatic tracking?", "[Y/n]:"))

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
		fmt.Println()
		fmt.Printf("  %s Hook configured in .claude/settings.json\n", style.Success(""))
		fmt.Printf("  %s Steps will be captured automatically\n", style.Success(""))
		fmt.Println()
		return nil
	}

	// User declined
	fmt.Println()
	fmt.Printf("  %s Skipped - you can configure manually later\n", style.DimText("⊘"))
	fmt.Println()
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

	// Check if hook already configured (handle both old and new formats)
	if hooks, ok := settings["hooks"].(map[string]interface{}); ok {
		// Old format: "PostToolUse": "rgt hook"
		if postToolUse, ok := hooks["PostToolUse"].(string); ok && postToolUse == "rgt hook" {
			fmt.Printf("%s PostToolUse hook already configured (old format - will upgrade)\n", style.Success(""))
			fmt.Println()
			// Don't return - let it upgrade to new format
		} else if postToolUseArray, ok := hooks["PostToolUse"].([]interface{}); ok {
			// New format: check if rgt hook exists
			for _, entry := range postToolUseArray {
				if entryMap, ok := entry.(map[string]interface{}); ok {
					if hooksArray, ok := entryMap["hooks"].([]interface{}); ok {
						for _, hook := range hooksArray {
							if hookMap, ok := hook.(map[string]interface{}); ok {
								if cmd, ok := hookMap["command"].(string); ok && strings.Contains(cmd, "rgt hook") {
									fmt.Printf("%s PostToolUse hook already configured\n", style.Success(""))
									fmt.Println()
									return nil
								}
							}
						}
					}
				}
			}
		}
	}

	// Add or update hooks section
	if settings["hooks"] == nil {
		settings["hooks"] = make(map[string]interface{})
	}

	hooks := settings["hooks"].(map[string]interface{})

	// Create hook entry in correct format (array with matcher + hooks)
	hookEntry := map[string]interface{}{
		"matcher": "",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": "rgt hook",
			},
		},
	}

	hooks["PostToolUse"] = []interface{}{hookEntry}

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
	fmt.Println("      \"PostToolUse\": [")
	fmt.Println("        {")
	fmt.Println("          \"matcher\": \"\",")
	fmt.Println("          \"hooks\": [")
	fmt.Println("            {")
	fmt.Println("              \"type\": \"command\",")
	fmt.Println("              \"command\": \"rgt hook\"")
	fmt.Println("            }")
	fmt.Println("          ]")
	fmt.Println("        }")
	fmt.Println("      ]")
	fmt.Println("    }")
	fmt.Println("  }")
	fmt.Println()
}

// createRegentGitignore creates .regent/.gitignore to ignore temporary files
func createRegentGitignore(projectRoot string) error {
	gitignorePath := filepath.Join(projectRoot, ".regent", ".gitignore")

	content := `# Regent temporary files
*.backup
rewound-*.jsonl
log/
`

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}

// offerSkillInstall prompts user and installs Claude skills if approved
func offerSkillInstall(projectRoot string) error {
	fmt.Printf("Claude skills let you use %s commands with slash syntax:\n", style.Brand("re_gent"))
	fmt.Println()
	fmt.Println("  /log [options]       Show step history")
	fmt.Println("  /blame <file>        Show line provenance")
	fmt.Println("  /show <step>         Show step details")
	fmt.Println("  /rewind <step>       Rewind to a step")
	fmt.Println()
	fmt.Print(style.Prompt("Install skills?", "[Y/n]:"))

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))

	// Default to yes
	if response == "" || response == "y" || response == "yes" {
		if err := installSkills(projectRoot); err != nil {
			return err
		}
		fmt.Println()
		fmt.Printf("  %s Skills installed in .claude/skills/\n", style.Success(""))
		fmt.Printf("  %s Use /log, /blame, /show, /rewind in Claude\n", style.Success(""))
		fmt.Println()
		return nil
	}

	fmt.Println()
	fmt.Printf("  %s Skipped - you can install manually later\n", style.DimText("⊘"))
	fmt.Println()
	return nil
}

// installSkills creates Claude skill files in .claude/skills/
func installSkills(projectRoot string) error {
	skillsDir := filepath.Join(projectRoot, ".claude", "skills")
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create skills directory: %w", err)
	}

	// Each skill needs its own directory with a SKILL.md file inside
	skills := map[string]string{
		"log": `---
description: View Regent activity log with step history, filtering, and formatting options. Use when reviewing agent session history, finding specific changes, or understanding what happened in previous steps.
allowed-tools: Bash(rgt log *)
argument-hint: "[filter-flags]"
---

Display the Regent activity log showing captured steps, tool calls, and conversation context.

Run the log command with any flags:
` + "```bash\nrgt log $ARGUMENTS\n```" + `

## Common usage

Show recent steps:
` + "```bash\nrgt log\n```" + `

Filter by session:
` + "```bash\nrgt log --session <session-id>\n```" + `

Change format:
` + "```bash\nrgt log --format timeline\nrgt log --format compact\n```",

		"blame": `---
description: Show which Regent step last modified each line of a file. Use when investigating file provenance, understanding change history, or debugging.
allowed-tools: Bash(rgt blame *)
argument-hint: "<file> [line]"
---

Display per-line provenance showing which step introduced or last modified each line.

Run blame on a file:
` + "```bash\nrgt blame $ARGUMENTS\n```" + `

## Examples

Blame entire file:
` + "```bash\nrgt blame src/main.go\n```" + `

Blame specific line:
` + "```bash\nrgt blame src/main.go:42\n```",

		"show": `---
description: Show detailed context for a Regent step including tool arguments, result, and conversation. Use when investigating what happened in a specific step.
allowed-tools: Bash(rgt show *)
argument-hint: "<step-hash>"
---

Display full details for a step including:
- Tool call (name, arguments)
- Tool result
- Conversation messages
- File changes

Show step details:
` + "```bash\nrgt show $ARGUMENTS\n```" + `

The step hash can be shortened (first 7+ characters).`,

		"rewind": `---
description: Rewind workspace and conversation to a previous Regent step. Non-destructive with automatic backup. Use when recovering from mistakes or exploring alternative paths.
allowed-tools: Bash(rgt rewind *)
argument-hint: "<step-hash>"
disable-model-invocation: true
---

⚠️ **Warning**: This will restore files and conversation state to the specified step.

Automatic backup is created at ` + "`" + `.regent/backups/` + "`" + ` before rewinding.

Rewind to a step:
` + "```bash\nrgt rewind $ARGUMENTS\n```" + `

The step hash can be shortened (first 7+ characters).

After rewinding, the current conversation transcript will be saved and the workspace files will match the target step's state.`,
	}

	for skillName, content := range skills {
		// Create skill directory
		skillDir := filepath.Join(skillsDir, skillName)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return fmt.Errorf("create skill directory %s: %w", skillName, err)
		}

		// Write SKILL.md inside the directory
		skillPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write skill %s: %w", skillName, err)
		}
	}

	return nil
}
