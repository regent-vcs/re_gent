package store

import (
	"fmt"
	"os"
	"path/filepath"
)

// Hash is a BLAKE3 hash hex-encoded
type Hash string

// Store manages the .regent object store
type Store struct {
	Root string // path to .regent directory
}

// Open opens an existing .regent store
func Open(regentDir string) (*Store, error) {
	if _, err := os.Stat(regentDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("regent store not found at %s (run 'rgt init')", regentDir)
	}

	s := &Store{Root: regentDir}

	// Verify required directories exist
	for _, dir := range []string{"objects", "refs/sessions"} {
		p := filepath.Join(regentDir, dir)
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	return s, nil
}

// Init creates a new .regent store in the given directory
func Init(workspaceRoot string) (*Store, error) {
	regentDir := filepath.Join(workspaceRoot, ".regent")

	if _, err := os.Stat(regentDir); err == nil {
		return nil, fmt.Errorf(".regent/ already exists in %s\n  Use 'rgt status' to check existing repository", workspaceRoot)
	}

	// Create directory structure
	dirs := []string{
		"objects",
		"refs/sessions",
		"log",
	}

	for _, dir := range dirs {
		p := filepath.Join(regentDir, dir)
		if err := os.MkdirAll(p, 0o755); err != nil {
			return nil, fmt.Errorf("create directory %s: %w", dir, err)
		}
	}

	// Create empty config file
	configPath := filepath.Join(regentDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("# Regent configuration\n"), 0o644); err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}

	return Open(regentDir)
}
