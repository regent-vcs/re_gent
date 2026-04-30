package ignore

import (
	"os"
	"path/filepath"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// Matcher matches paths against ignore patterns
type Matcher struct {
	gi *gitignore.GitIgnore
}

// Default creates a matcher with default ignore patterns
func Default(workspaceRoot string) *Matcher {
	patterns := []string{
		"node_modules/",
		".git/",
		".regent/",
		"__pycache__/",
		"*.pyc",
		".venv/",
		"venv/",
		"target/",
		"dist/",
		"build/",
		".next/",
		".cache/",
	}

	// Try to load .regentignore from workspace root
	ignorePath := filepath.Join(workspaceRoot, ".regentignore")
	if data, err := os.ReadFile(ignorePath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				patterns = append(patterns, line)
			}
		}
	}

	return &Matcher{
		gi: gitignore.CompileIgnoreLines(patterns...),
	}
}

// Match returns true if the path should be ignored
// path should be relative to workspace root
// isDir indicates if the path is a directory
func (m *Matcher) Match(path string, isDir bool) bool {
	if isDir {
		// gitignore treats directories with trailing slash
		path = path + "/"
	}
	return m.gi.MatchesPath(path)
}
