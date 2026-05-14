package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/pelletier/go-toml/v2"
	"github.com/regent-vcs/regent/internal/index"
	"github.com/regent-vcs/regent/internal/store"
	"github.com/regent-vcs/regent/internal/style"
	"github.com/spf13/cobra"
)

type agentTarget string

type hookInstallResult struct {
	BackupPath string
}

const (
	agentAuto     agentTarget = "auto"
	agentClaude   agentTarget = "claude"
	agentCodex    agentTarget = "codex"
	agentOpenCode agentTarget = "opencode"
	agentBoth     agentTarget = "both"
	agentAll      agentTarget = "all"

	claudeUserHook      = "rgt message-hook user"
	claudeAssistantHook = "rgt message-hook assistant"
	claudeToolBatchHook = "rgt tool-batch-hook"
	codexHookCommand    = "rgt codex-hook"
)

func InitCmd() *cobra.Command {
	var skipHook bool
	var skipSkills bool
	var agent string

	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Initialize a new re_gent repository",
		Long:         "Creates a .regent directory in the current workspace and sets up agent hooks.",
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandOrder": "0",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			targets, err := resolveAgentTargets(cwd, agentTarget(agent))
			if err != nil {
				return err
			}
			input := bufio.NewReader(os.Stdin)

			printHeader()

			reinit := pathExists(filepath.Join(cwd, ".regent"))

			printStep(1, 3, "Initialize Repository")
			if reinit {
				s, err := store.Open(filepath.Join(cwd, ".regent"))
				if err != nil {
					return err
				}
				idx, err := index.Open(s)
				if err != nil {
					return fmt.Errorf("open index: %w", err)
				}
				defer func() { _ = idx.Close() }()

				fmt.Printf("  %s .regent/ already exists (skipping creation)\n", style.DimText("-"))
				fmt.Println()
			} else {
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
			}

			printStep(2, 3, "Configure Agent Hooks")
			if reinit {
				printExistingHooks(cwd)
			}
			installedTargets := targets
			if skipHook {
				fmt.Printf("  %s Hook configuration skipped\n", style.DimText("-"))
				printManualInstructions(targets)
			} else {
				selected, err := offerHookInstall(cwd, targets, input)
				if err != nil {
					fmt.Printf("  %s Could not configure hooks: %v\n", style.Warning(""), err)
					printManualInstructions(targets)
				} else if len(selected) > 0 {
					installedTargets = selected
				}
			}

			printStep(3, 3, "Install Agent Skills")
			if skipSkills {
				fmt.Printf("  %s Skill installation skipped\n", style.DimText("-"))
			} else if err := offerSkillInstall(cwd, installedTargets, input); err != nil {
				fmt.Printf("  %s Could not install skills: %v\n", style.Warning(""), err)
			}

			printSummary(cwd, targets)
			return nil
		},
	}

	cmd.Flags().BoolVar(&skipHook, "skip-hook", false, "Skip automatic hook configuration")
	cmd.Flags().BoolVar(&skipSkills, "skip-skills", false, "Skip agent skill installation")
	cmd.Flags().StringVar(&agent, "agent", string(agentAuto), "Agent hooks to configure: auto, claude, codex, opencode, all")

	return cmd
}

func printHeader() {
	fmt.Println()
	fmt.Println(style.DividerFull(""))
	fmt.Printf("  %s - Version Control for AI Agent Activity\n", style.Brand("re_gent"))
	fmt.Println(style.DividerFull(""))
	fmt.Println()
}

func printStep(current, total int, title string) {
	fmt.Println(style.SectionHeader(fmt.Sprintf("Step %d/%d: %s", current, total, title)))
	fmt.Println()
}

func printSummary(projectRoot string, targets []agentTarget) {
	fmt.Println()
	fmt.Println(style.DividerFull(""))
	fmt.Printf("  %s Initialization complete\n", style.Success(""))
	fmt.Println(style.DividerFull(""))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  - Start an agent session in this directory")
	fmt.Println("  - Make changes with Claude Code, Codex, or OpenCode")
	fmt.Println("  - Run: rgt log")
	fmt.Println("  - Run: rgt blame <file>")
	if hasAgent(targets, agentClaude) || hasAgent(targets, agentCodex) || hasAgent(targets, agentOpenCode) {
		fmt.Println("  - Agent skills: log, blame, show")
	}
	if hasAgent(targets, agentCodex) {
		fmt.Println("  - Codex may ask you to trust this project and the re_gent hooks")
	}
	fmt.Println()
	fmt.Printf("%s %s\n", style.Label("Repository:"), filepath.Join(projectRoot, ".regent"))
	fmt.Println()
}

func offerHookInstall(projectRoot string, targets []agentTarget, _ *bufio.Reader) ([]agentTarget, error) {
	fmt.Printf("%s captures step history automatically via agent hooks.\n", style.Brand("re_gent"))
	fmt.Println()

	options := make([]huh.Option[agentTarget], 0, len(targets))
	for _, target := range targets {
		switch target {
		case agentClaude:
			options = append(options, huh.NewOption("Claude Code   (.claude/settings.json)", agentClaude))
		case agentCodex:
			options = append(options, huh.NewOption("Codex         (.codex/config.toml)", agentCodex))
		case agentOpenCode:
			options = append(options, huh.NewOption("OpenCode      (opencode.jsonc + npm plugin)", agentOpenCode))
		}
	}

	var selected []agentTarget
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[agentTarget]().
				Title("Select agents to configure").
				Description("Use arrow keys to navigate, space to toggle, enter to confirm").
				Options(options...).
				Value(&selected),
		),
	)

	if err := form.Run(); err != nil {
		return nil, fmt.Errorf("agent selection: %w", err)
	}

	if len(selected) == 0 {
		fmt.Printf("  %s Skipped - you can configure hooks manually later\n", style.DimText("-"))
		fmt.Println()
		return nil, nil
	}

	for _, target := range selected {
		switch target {
		case agentClaude:
			result, err := installClaudeHook(projectRoot)
			if err != nil {
				return nil, err
			}
			printHookInstallWarning(result)
			fmt.Printf("  %s Claude Code hooks configured\n", style.Success(""))
		case agentCodex:
			result, err := installCodexHook(projectRoot)
			if err != nil {
				return nil, err
			}
			printHookInstallWarning(result)
			fmt.Printf("  %s Codex hooks configured\n", style.Success(""))
		case agentOpenCode:
			if err := installOpenCodeHook(projectRoot); err != nil {
				return nil, err
			}
			fmt.Printf("  %s OpenCode plugin installed\n", style.Success(""))
		}
	}
	fmt.Println()
	return selected, nil
}

func printHookInstallWarning(result hookInstallResult) {
	if result.BackupPath == "" {
		return
	}
	fmt.Printf("  %s Existing hook config was invalid; backed up to %s before rewriting\n", style.Warning(""), result.BackupPath)
}

func installClaudeHook(projectRoot string) (hookInstallResult, error) {
	var result hookInstallResult
	claudeDir := filepath.Join(projectRoot, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return result, fmt.Errorf("create .claude directory: %w", err)
	}

	settings := map[string]interface{}{}
	if data, err := os.ReadFile(settingsPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &settings); err != nil {
			backupPath, err := backupFile(settingsPath)
			if err != nil {
				return result, fmt.Errorf("backup invalid Claude settings: %w", err)
			}
			result.BackupPath = backupPath
			settings = map[string]interface{}{}
		}
	}

	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
		settings["hooks"] = hooks
	}

	mergeHookCommand(hooks, "UserPromptSubmit", claudeUserHook)
	mergeHookCommand(hooks, "Stop", claudeAssistantHook)
	mergeHookCommand(hooks, "PostToolBatch", claudeToolBatchHook)
	removeRegentHookCommands(hooks, "PostToolUse")

	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return result, fmt.Errorf("marshal Claude settings: %w", err)
	}
	if err := os.WriteFile(settingsPath, output, 0o644); err != nil {
		return result, fmt.Errorf("write Claude settings: %w", err)
	}

	return result, nil
}

func installCodexHook(projectRoot string) (hookInstallResult, error) {
	var result hookInstallResult
	codexDir := filepath.Join(projectRoot, ".codex")
	configPath := filepath.Join(codexDir, "config.toml")

	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return result, fmt.Errorf("create .codex directory: %w", err)
	}

	config := map[string]interface{}{}
	if data, err := os.ReadFile(configPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		if err := toml.Unmarshal(data, &config); err != nil {
			backupPath, err := backupFile(configPath)
			if err != nil {
				return result, fmt.Errorf("backup invalid Codex config: %w", err)
			}
			result.BackupPath = backupPath
			config = map[string]interface{}{}
		}
	}

	hooks, _ := config["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = map[string]interface{}{}
		config["hooks"] = hooks
	}

	for _, eventName := range []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "Stop"} {
		mergeHookCommand(hooks, eventName, codexHookCommand)
	}
	enableCodexHooksFeature(config)

	output, err := toml.Marshal(config)
	if err != nil {
		return result, fmt.Errorf("marshal Codex config: %w", err)
	}
	if err := os.WriteFile(configPath, output, 0o644); err != nil {
		return result, fmt.Errorf("write Codex config: %w", err)
	}

	return result, nil
}

func installOpenCodeHook(projectRoot string) error {
	opencodeDir := filepath.Join(projectRoot, ".opencode")
	if err := os.MkdirAll(opencodeDir, 0o755); err != nil {
		return fmt.Errorf("create .opencode directory: %w", err)
	}

	if err := npmInstallOpenCodePlugin(opencodeDir); err != nil {
		return err
	}

	return registerOpenCodePlugin(projectRoot)
}

func npmInstallOpenCodePlugin(opencodeDir string) error {
	fmt.Printf("  %s Installing @regent-vcs/opencode-plugin...\n", style.DimText("⟳"))
	cmd := exec.Command("npm", "install", "--save", "@regent-vcs/opencode-plugin")
	cmd.Dir = opencodeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("npm install @regent-vcs/opencode-plugin: %w", err)
	}
	return nil
}

func registerOpenCodePlugin(projectRoot string) error {
	configPath := findOpenCodeConfig(projectRoot)
	if configPath == "" {
		configPath = filepath.Join(projectRoot, "opencode.jsonc")
	}

	config := map[string]interface{}{}
	if data, err := os.ReadFile(configPath); err == nil && len(strings.TrimSpace(string(data))) > 0 {
		cleaned := stripJSONComments(string(data))
		if err := json.Unmarshal([]byte(cleaned), &config); err != nil {
			config = map[string]interface{}{}
		}
	}

	pluginRef := "@regent-vcs/opencode-plugin"
	plugins, _ := config["plugin"].([]interface{})
	for _, p := range plugins {
		if s, ok := p.(string); ok && s == pluginRef {
			return nil
		}
	}
	config["plugin"] = append(plugins, pluginRef)

	output, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal OpenCode config: %w", err)
	}
	return os.WriteFile(configPath, output, 0o644)
}

func findOpenCodeConfig(projectRoot string) string {
	candidates := []string{
		filepath.Join(projectRoot, "opencode.jsonc"),
		filepath.Join(projectRoot, "opencode.json"),
		filepath.Join(projectRoot, ".opencode.jsonc"),
		filepath.Join(projectRoot, ".opencode.json"),
	}
	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func stripJSONComments(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
		} else if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) && !(s[i] == '*' && s[i+1] == '/') {
				i++
			}
			if i+1 < len(s) {
				i += 2
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func enableCodexHooksFeature(config map[string]interface{}) {
	features, _ := config["features"].(map[string]interface{})
	if features == nil {
		features = map[string]interface{}{}
		config["features"] = features
	}
	features["hooks"] = true
}

func mergeHookCommand(hooks map[string]interface{}, eventName, command string) {
	groups := filterRegentHookCommands(normalizeHookGroups(hooks[eventName]))
	hooks[eventName] = append(groups, hookGroup(command))
}

func removeRegentHookCommands(hooks map[string]interface{}, eventName string) {
	groups := filterRegentHookCommands(normalizeHookGroups(hooks[eventName]))
	if len(groups) == 0 {
		delete(hooks, eventName)
		return
	}
	hooks[eventName] = groups
}

func normalizeHookGroups(value interface{}) []interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case []interface{}:
		return typed
	case []map[string]interface{}:
		groups := make([]interface{}, 0, len(typed))
		for _, group := range typed {
			groups = append(groups, group)
		}
		return groups
	case map[string]interface{}:
		return []interface{}{typed}
	case string:
		return []interface{}{hookGroup(typed)}
	default:
		return []interface{}{typed}
	}
}

func filterRegentHookCommands(groups []interface{}) []interface{} {
	filtered := make([]interface{}, 0, len(groups))
	for _, group := range groups {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			filtered = append(filtered, group)
			continue
		}

		hookEntries, hasHooks := normalizeHookEntries(groupMap["hooks"])
		if !hasHooks {
			filtered = append(filtered, group)
			continue
		}

		nextHookEntries := make([]interface{}, 0, len(hookEntries))
		for _, hookEntry := range hookEntries {
			hookMap, ok := hookEntry.(map[string]interface{})
			if !ok {
				nextHookEntries = append(nextHookEntries, hookEntry)
				continue
			}
			command, _ := hookMap["command"].(string)
			if isRegentHookCommand(command) {
				continue
			}
			nextHookEntries = append(nextHookEntries, hookEntry)
		}
		if len(nextHookEntries) == 0 {
			continue
		}

		nextGroup := map[string]interface{}{}
		for key, value := range groupMap {
			nextGroup[key] = value
		}
		nextGroup["hooks"] = nextHookEntries
		filtered = append(filtered, nextGroup)
	}
	return filtered
}

func normalizeHookEntries(value interface{}) ([]interface{}, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case []interface{}:
		return typed, true
	case []map[string]interface{}:
		entries := make([]interface{}, 0, len(typed))
		for _, entry := range typed {
			entries = append(entries, entry)
		}
		return entries, true
	case map[string]interface{}:
		return []interface{}{typed}, true
	default:
		return []interface{}{typed}, true
	}
}

func hookGroup(command string) map[string]interface{} {
	return map[string]interface{}{
		"matcher": "",
		"hooks": []interface{}{
			map[string]interface{}{
				"type":    "command",
				"command": command,
			},
		},
	}
}

func isRegentHookCommand(command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	for len(fields) > 0 && strings.Contains(fields[0], "=") && !strings.HasPrefix(fields[0], "=") {
		fields = fields[1:]
	}
	if len(fields) == 0 {
		return false
	}

	first := strings.TrimPrefix(filepath.Base(fields[0]), "./")
	if first == "rgt" || first == "regent" {
		return true
	}

	return len(fields) >= 3 && fields[0] == "go" && fields[1] == "run" && strings.Contains(fields[2], "cmd/rgt")
}

func printExistingHooks(projectRoot string) {
	fmt.Println("Currently configured:")
	fmt.Println()

	settingsPath := filepath.Join(projectRoot, ".claude", "settings.json")
	if data, err := os.ReadFile(settingsPath); err == nil {
		var settings map[string]interface{}
		if json.Unmarshal(data, &settings) == nil {
			if hooks, ok := settings["hooks"].(map[string]interface{}); ok && len(hooks) > 0 {
				for event := range hooks {
					groups := normalizeHookGroups(hooks[event])
					for _, g := range groups {
						if gm, ok := g.(map[string]interface{}); ok {
							entries, _ := normalizeHookEntries(gm["hooks"])
							for _, e := range entries {
								if em, ok := e.(map[string]interface{}); ok {
									if cmd, _ := em["command"].(string); isRegentHookCommand(cmd) {
										fmt.Printf("  %s Claude Code\n", style.Success(""))
										goto doneClaudeCheck
									}
								}
							}
						}
					}
				}
			}
		}
	}
doneClaudeCheck:

	configPath := filepath.Join(projectRoot, ".codex", "config.toml")
	if data, err := os.ReadFile(configPath); err == nil {
		var config map[string]interface{}
		if toml.Unmarshal(data, &config) == nil {
			if hooks, ok := config["hooks"].(map[string]interface{}); ok {
				for event := range hooks {
					groups := normalizeHookGroups(hooks[event])
					for _, g := range groups {
						if gm, ok := g.(map[string]interface{}); ok {
							entries, _ := normalizeHookEntries(gm["hooks"])
							for _, e := range entries {
								if em, ok := e.(map[string]interface{}); ok {
									if cmd, _ := em["command"].(string); isRegentHookCommand(cmd) {
										fmt.Printf("  %s Codex\n", style.Success(""))
										goto doneCodexCheck
									}
								}
							}
						}
					}
				}
			}
		}
	}
doneCodexCheck:

	opencodeConfig := findOpenCodeConfig(projectRoot)
	if opencodeConfig != "" {
		if data, err := os.ReadFile(opencodeConfig); err == nil {
			cleaned := stripJSONComments(string(data))
			var config map[string]interface{}
			if json.Unmarshal([]byte(cleaned), &config) == nil {
				if plugins, ok := config["plugin"].([]interface{}); ok {
					for _, p := range plugins {
						if s, ok := p.(string); ok && strings.Contains(s, "regent") {
							fmt.Printf("  %s OpenCode\n", style.Success(""))
							break
						}
					}
				}
			}
		}
	}

	fmt.Println()
}

func printManualInstructions(targets []agentTarget) {
	fmt.Println("Manual hook configuration:")
	fmt.Println()
	if hasAgent(targets, agentClaude) {
		fmt.Println("Claude Code .claude/settings.json events:")
		fmt.Println("  UserPromptSubmit -> rgt message-hook user")
		fmt.Println("  Stop             -> rgt message-hook assistant")
		fmt.Println("  PostToolBatch    -> rgt tool-batch-hook")
		fmt.Println()
	}
	if hasAgent(targets, agentCodex) {
		fmt.Println("Codex .codex/config.toml events:")
		fmt.Println("  SessionStart     -> rgt codex-hook")
		fmt.Println("  UserPromptSubmit -> rgt codex-hook")
		fmt.Println("  PostToolUse      -> rgt codex-hook")
		fmt.Println("  Stop             -> rgt codex-hook")
		fmt.Println()
	}
	if hasAgent(targets, agentOpenCode) {
		fmt.Println("OpenCode: copy the re_gent plugin to .opencode/plugins/regent.ts")
		fmt.Println("  The plugin bridges tool.execute.after and session.idle to rgt opencode-hook")
		fmt.Println()
	}
}

func createRegentGitignore(projectRoot string) error {
	gitignorePath := filepath.Join(projectRoot, ".regent", ".gitignore")
	content := `# re_gent temporary files
*.backup
log/
`

	return os.WriteFile(gitignorePath, []byte(content), 0o644)
}

func offerSkillInstall(projectRoot string, targets []agentTarget, input *bufio.Reader) error {
	fmt.Printf("Agent skills expose common %s commands inside the agent UI.\n", style.Brand("re_gent"))
	fmt.Println()
	fmt.Println("  log [options]         Show step history")
	fmt.Println("  blame <path>[:<line>] Show line provenance")
	fmt.Println("  show <step>           Show step details")
	fmt.Println()
	fmt.Print(style.Prompt("Install skills?", "[Y/n]:"))

	confirmed, err := confirmedDefaultYes(input)
	if err != nil {
		return fmt.Errorf("read skill confirmation: %w", err)
	}
	if !confirmed {
		fmt.Println()
		fmt.Printf("  %s Skipped - you can install skills manually later\n", style.DimText("-"))
		fmt.Println()
		return nil
	}

	for _, target := range targets {
		switch target {
		case agentClaude:
			if err := installSkills(filepath.Join(projectRoot, ".claude", "skills")); err != nil {
				return err
			}
			fmt.Printf("  %s Claude skills installed in .claude/skills/\n", style.Success(""))
		case agentCodex:
			if err := installSkills(filepath.Join(projectRoot, ".agents", "skills")); err != nil {
				return err
			}
			fmt.Printf("  %s Codex skills installed in .agents/skills/\n", style.Success(""))
		case agentOpenCode:
			if err := installSkills(filepath.Join(projectRoot, ".opencode", "skills")); err != nil {
				return err
			}
			fmt.Printf("  %s OpenCode skills installed in .opencode/skills/\n", style.Success(""))
		}
	}
	fmt.Println()
	return nil
}

func installSkills(skillsDir string) error {
	if err := os.MkdirAll(skillsDir, 0o755); err != nil {
		return fmt.Errorf("create skills directory: %w", err)
	}

	for skillName, content := range skillContents() {
		skillDir := filepath.Join(skillsDir, skillName)
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			return fmt.Errorf("create skill directory %s: %w", skillName, err)
		}

		skillPath := filepath.Join(skillDir, "SKILL.md")
		if err := os.WriteFile(skillPath, []byte(content), 0o644); err != nil {
			return fmt.Errorf("write skill %s: %w", skillName, err)
		}
	}

	return nil
}

func skillContents() map[string]string {
	return map[string]string{
		"log": `---
description: View the re_gent activity log for the default or selected session. The default view shows the conversation timeline and tool calls; file summaries are available with file flags.
allowed-tools: Bash(rgt log *)
argument-hint: "[session-id] [flags]"
---

Display the re_gent activity log.

By default, ` + "`rgt log`" + ` shows the conversation timeline for the most recent session with captured steps. Use ` + "`--files-only`" + ` for file-change summaries.

Run:
` + "```bash\nrgt log $ARGUMENTS\n```" + `

Common usage:
` + "```bash\nrgt log\nrgt log --conversation-only\nrgt log --files-only\nrgt log --graph\nrgt log --limit 50\n```",

		"blame": `---
description: Show which re_gent step last modified each line of a file. Use when investigating file provenance or debugging.
allowed-tools: Bash(rgt blame *)
argument-hint: "<path>[:<line>]"
---

Display per-line provenance.

Run:
` + "```bash\nrgt blame $ARGUMENTS\n```",

		"show": `---
description: Show detailed context for a re_gent step, including tool calls, tool results, and conversation.
allowed-tools: Bash(rgt show *)
argument-hint: "<step-hash>"
---

Display full details for a step.

Run:
` + "```bash\nrgt show $ARGUMENTS\n```",
	}
}

func resolveAgentTargets(projectRoot string, target agentTarget) ([]agentTarget, error) {
	switch target {
	case agentClaude:
		return []agentTarget{agentClaude}, nil
	case agentCodex:
		return []agentTarget{agentCodex}, nil
	case agentOpenCode:
		return []agentTarget{agentOpenCode}, nil
	case agentBoth:
		return []agentTarget{agentClaude, agentCodex}, nil
	case agentAll:
		return []agentTarget{agentClaude, agentCodex, agentOpenCode}, nil
	case agentAuto, "":
		var targets []agentTarget
		if pathExists(filepath.Join(projectRoot, ".claude")) || commandExists("claude") {
			targets = append(targets, agentClaude)
		}
		if pathExists(filepath.Join(projectRoot, ".codex")) || commandExists("codex") {
			targets = append(targets, agentCodex)
		}
		if pathExists(filepath.Join(projectRoot, ".opencode")) || commandExists("opencode") {
			targets = append(targets, agentOpenCode)
		}
		if len(targets) == 0 {
			targets = append(targets, agentClaude, agentCodex)
		}
		return targets, nil
	default:
		return nil, fmt.Errorf("invalid --agent %q; expected auto, claude, codex, opencode, all, or both", target)
	}
}

func hasAgent(targets []agentTarget, target agentTarget) bool {
	for _, candidate := range targets {
		if candidate == target {
			return true
		}
	}
	return false
}

func confirmedDefaultYes(input *bufio.Reader) (bool, error) {
	response, err := input.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "" || response == "y" || response == "yes", nil
}

func backupFile(path string) (string, error) {
	backupPath := path + ".backup"
	return backupPath, os.Rename(path, backupPath)
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
