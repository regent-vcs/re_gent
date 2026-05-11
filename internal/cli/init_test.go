package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestInstallCodexHook_MergesProjectConfig(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("model = \"gpt-5.5\"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := installCodexHook(root); err != nil {
		t.Fatalf("install hook: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if config["model"] != "gpt-5.5" {
		t.Fatalf("model was not preserved: %#v", config["model"])
	}

	features, ok := config["features"].(map[string]interface{})
	if !ok {
		t.Fatalf("features missing: %#v", config["features"])
	}
	if features["codex_hooks"] != true {
		t.Fatalf("codex_hooks feature not enabled: %#v", features["codex_hooks"])
	}

	hooks, ok := config["hooks"].(map[string]interface{})
	if !ok {
		t.Fatalf("hooks missing: %#v", config["hooks"])
	}
	for _, eventName := range []string{"SessionStart", "UserPromptSubmit", "PostToolUse", "Stop"} {
		if hooks[eventName] == nil {
			t.Fatalf("hook %s missing", eventName)
		}
	}
}

func TestInstallCodexHook_PreservesExistingFeatures(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	existing := `
[features]
web_search = true
`
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := installCodexHook(root); err != nil {
		t.Fatalf("install hook: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	features := config["features"].(map[string]interface{})
	if features["web_search"] != true {
		t.Fatalf("existing feature was not preserved: %#v", features)
	}
	if features["codex_hooks"] != true {
		t.Fatalf("codex_hooks feature not enabled: %#v", features)
	}
}

func TestInstallCodexHook_PreservesExistingHooks(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	existing := `
[hooks]
[[hooks.PostToolUse]]
matcher = "Bash"
[[hooks.PostToolUse.hooks]]
type = "command"
command = "echo keep"

[[hooks.PostToolUse]]
matcher = ""
[[hooks.PostToolUse.hooks]]
type = "command"
command = "rgt codex-hook"
`
	if err := os.WriteFile(configPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := installCodexHook(root); err != nil {
		t.Fatalf("install hook: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var config map[string]interface{}
	if err := toml.Unmarshal(data, &config); err != nil {
		t.Fatalf("parse config: %v", err)
	}
	commands := hookCommands(t, config["hooks"].(map[string]interface{})["PostToolUse"])
	if countCommand(commands, "echo keep") != 1 {
		t.Fatalf("expected existing hook to be preserved once, got %#v", commands)
	}
	if countCommand(commands, codexHookCommand) != 1 {
		t.Fatalf("expected one re_gent hook, got %#v", commands)
	}
}

func TestInstallClaudeHook_PreservesExistingHooksAndRemovesLegacyHook(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	existing := `{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "echo keep"}
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [
          {"type": "command", "command": "rgt hook"},
          {"type": "command", "command": "echo keep-post"}
        ]
      }
    ]
  }
}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	if _, err := installClaudeHook(root); err != nil {
		t.Fatalf("install hook: %v", err)
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("parse settings: %v", err)
	}
	hooks := settings["hooks"].(map[string]interface{})
	stopCommands := hookCommands(t, hooks["Stop"])
	if countCommand(stopCommands, "echo keep") != 1 || countCommand(stopCommands, claudeAssistantHook) != 1 {
		t.Fatalf("expected existing Stop hook and assistant hook, got %#v", stopCommands)
	}

	postToolUseCommands := hookCommands(t, hooks["PostToolUse"])
	if countCommand(postToolUseCommands, "rgt hook") != 0 || countCommand(postToolUseCommands, "echo keep-post") != 1 {
		t.Fatalf("expected legacy re_gent hook removed and unrelated hook kept, got %#v", postToolUseCommands)
	}
}

func TestInstallCodexHook_BacksUpInvalidConfig(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".codex")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[broken\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	result, err := installCodexHook(root)
	if err != nil {
		t.Fatalf("install hook: %v", err)
	}
	if result.BackupPath != configPath+".backup" {
		t.Fatalf("backup path = %q, want %q", result.BackupPath, configPath+".backup")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("expected backup: %v", err)
	}
}

func TestInstallClaudeHook_BacksUpInvalidSettings(t *testing.T) {
	root := t.TempDir()
	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{broken\n"), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	result, err := installClaudeHook(root)
	if err != nil {
		t.Fatalf("install hook: %v", err)
	}
	if result.BackupPath != settingsPath+".backup" {
		t.Fatalf("backup path = %q, want %q", result.BackupPath, settingsPath+".backup")
	}
	if _, err := os.Stat(result.BackupPath); err != nil {
		t.Fatalf("expected backup: %v", err)
	}
}

func TestInstallSkills_OmitsUnsupportedRewind(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".agents", "skills")
	if err := installSkills(skillsDir); err != nil {
		t.Fatalf("install skills: %v", err)
	}

	for _, skillName := range []string{"log", "blame", "show"} {
		if _, err := os.Stat(filepath.Join(skillsDir, skillName, "SKILL.md")); err != nil {
			t.Fatalf("expected %s skill: %v", skillName, err)
		}
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "rewind", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("rewind skill should not be installed, stat err=%v", err)
	}
}

func hookCommands(t *testing.T, value interface{}) []string {
	t.Helper()

	var commands []string
	for _, group := range normalizeHookGroups(value) {
		groupMap, ok := group.(map[string]interface{})
		if !ok {
			continue
		}
		hooks, _ := normalizeHookEntries(groupMap["hooks"])
		for _, hook := range hooks {
			hookMap, ok := hook.(map[string]interface{})
			if !ok {
				continue
			}
			command, _ := hookMap["command"].(string)
			commands = append(commands, command)
		}
	}
	return commands
}

func countCommand(commands []string, expected string) int {
	count := 0
	for _, command := range commands {
		if command == expected {
			count++
		}
	}
	return count
}
