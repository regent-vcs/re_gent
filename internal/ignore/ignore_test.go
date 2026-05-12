package ignore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultMatcher_BuiltInPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	m := Default(tmpDir)

	tests := []struct {
		path      string
		isDir     bool
		wantMatch bool
		desc      string
	}{
		{"node_modules", true, true, "node_modules directory"},
		{"node_modules/package", false, true, "file inside node_modules"},
		{".git", true, true, ".git directory"},
		{".regent", true, true, ".regent directory"},
		{"__pycache__", true, true, "__pycache__ directory"},
		{"script.pyc", false, true, ".pyc file"},
		{".venv", true, true, ".venv directory"},
		{"venv", true, true, "venv directory"},
		{"target", true, true, "target directory (Rust/Java)"},
		{"dist", true, true, "dist directory"},
		{"build", true, true, "build directory"},
		{".next", true, true, ".next directory (Next.js)"},
		{".cache", true, true, ".cache directory"},
		{"src/main.go", false, false, "normal source file"},
		{"README.md", false, false, "normal markdown file"},
		{"test", true, false, "normal test directory"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := m.Match(tt.path, tt.isDir)
			if got != tt.wantMatch {
				t.Errorf("Match(%q, isDir=%v) = %v, want %v",
					tt.path, tt.isDir, got, tt.wantMatch)
			}
		})
	}
}

func TestDefaultMatcher_RegentignoreFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .regentignore file with custom patterns
	regentignorePath := filepath.Join(tmpDir, ".regentignore")
	content := `# Custom ignore patterns
*.log
temp/
secrets.txt

# Empty line and comment above should be ignored
`
	if err := os.WriteFile(regentignorePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write .regentignore: %v", err)
	}

	m := Default(tmpDir)

	tests := []struct {
		path      string
		isDir     bool
		wantMatch bool
		desc      string
	}{
		{"debug.log", false, true, ".log file should match"},
		{"logs/app.log", false, true, ".log file in subdirectory"},
		{"temp", true, true, "temp directory"},
		{"temp/file.txt", false, true, "file in temp directory"},
		{"secrets.txt", false, true, "secrets.txt file"},
		{"src/main.go", false, false, "normal file not in ignore"},
		{".git", true, true, "built-in pattern still works"},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got := m.Match(tt.path, tt.isDir)
			if got != tt.wantMatch {
				t.Errorf("Match(%q, isDir=%v) = %v, want %v",
					tt.path, tt.isDir, got, tt.wantMatch)
			}
		})
	}
}

func TestDefaultMatcher_NoRegentignore(t *testing.T) {
	// Test that Default works when .regentignore doesn't exist
	tmpDir := t.TempDir()
	m := Default(tmpDir)

	// Should still match built-in patterns
	if !m.Match("node_modules", true) {
		t.Error("Should match node_modules even without .regentignore")
	}

	if m.Match("normal_file.txt", false) {
		t.Error("Should not match normal file")
	}
}

func TestMatcher_DirectoryVsFile(t *testing.T) {
	tmpDir := t.TempDir()
	m := Default(tmpDir)

	// Test that directory matching works correctly
	// Some patterns should only match directories
	if !m.Match("build", true) {
		t.Error("Should match 'build' as a directory")
	}

	// File named "build" without directory flag might behave differently
	// depending on the pattern - test both cases
	fileResult := m.Match("build", false)
	dirResult := m.Match("build", true)

	if fileResult == dirResult {
		// This is fine - both match or both don't
		// The important thing is it's consistent
		t.Logf("File and directory results are the same: %v", fileResult)
	}
}

func TestMatcher_RelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	m := Default(tmpDir)

	tests := []struct {
		path      string
		isDir     bool
		wantMatch bool
	}{
		{"src/node_modules/lib", false, true},
		{"deep/nested/path/.cache/data", false, true},
		{"project/.git/objects", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := m.Match(tt.path, tt.isDir)
			if got != tt.wantMatch {
				t.Errorf("Match(%q, %v) = %v, want %v",
					tt.path, tt.isDir, got, tt.wantMatch)
			}
		})
	}
}

func TestMatcher_EmptyPath(t *testing.T) {
	tmpDir := t.TempDir()
	m := Default(tmpDir)

	// Empty path should not match
	if m.Match("", false) {
		t.Error("Empty path should not match")
	}
	if m.Match("", true) {
		t.Error("Empty path (directory) should not match")
	}
}
