package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVersionStringIncludesVersionAndCommit(t *testing.T) {
	defer restoreVersionVars(Version, Commit, Date)
	Version, Commit, Date = "1.2.3", "abcdef0", ""

	got := VersionString()
	if !strings.Contains(got, "1.2.3") {
		t.Errorf("VersionString() = %q, want it to contain version %q", got, "1.2.3")
	}
	if !strings.Contains(got, "abcdef0") {
		t.Errorf("VersionString() = %q, want it to contain commit %q", got, "abcdef0")
	}
	if strings.Contains(got, "built") {
		t.Errorf("VersionString() = %q, should not mention build date when Date is empty", got)
	}
}

func TestVersionStringIncludesDateWhenSet(t *testing.T) {
	defer restoreVersionVars(Version, Commit, Date)
	Version, Commit, Date = "1.2.3", "abcdef0", "2026-06-13T00:00:00Z"

	got := VersionString()
	if !strings.Contains(got, "2026-06-13T00:00:00Z") {
		t.Errorf("VersionString() = %q, want it to contain build date", got)
	}
}

// TestBuildConfigTargetsVersionSymbols is the regression guard for issue #64:
// released binaries reported "dev" because the linker -X flags targeted
// main.version (which does not exist) instead of these cli package symbols.
// If the symbol path ever drifts from the build config again, this fails.
func TestBuildConfigTargetsVersionSymbols(t *testing.T) {
	const symbolPath = "github.com/regent-vcs/regent/internal/cli."

	repoRoot := filepath.Join("..", "..")
	cases := []struct {
		file    string
		symbols []string
	}{
		{".goreleaser.yaml", []string{"Version", "Commit", "Date"}},
		{"Makefile", []string{"Version", "Commit", "Date"}},
	}

	for _, tc := range cases {
		data, err := os.ReadFile(filepath.Join(repoRoot, tc.file))
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		content := string(data)
		for _, sym := range tc.symbols {
			want := "-X " + symbolPath + sym
			if !strings.Contains(content, want) {
				t.Errorf("%s does not stamp %q via ldflags (want substring %q)", tc.file, sym, want)
			}
		}
	}
}

func restoreVersionVars(v, c, d string) {
	Version, Commit, Date = v, c, d
}
