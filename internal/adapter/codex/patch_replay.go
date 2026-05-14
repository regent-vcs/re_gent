package codex

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/regent-vcs/regent/internal/store"
)

var hunkHeaderRE = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

func applyPatchToEntries(s *store.Store, projectRoot string, base map[string]store.TreeEntry, patchText string) (map[string]store.TreeEntry, error) {
	lines := splitPatchLines(patchText)
	result := cloneEntries(base)

	for i := 0; i < len(lines); {
		line := lines[i]
		switch {
		case line == "" || line == "*** Begin Patch" || line == "*** End Patch":
			i++
		case strings.HasPrefix(line, "*** Add File: "):
			next, path, content, err := parseAddFileBlock(lines, i, projectRoot)
			if err != nil {
				return nil, err
			}
			blob, err := s.WriteBlob([]byte(strings.Join(content, "\n")))
			if err != nil {
				return nil, err
			}
			result[path] = store.TreeEntry{Path: path, Blob: blob, Mode: 0o644}
			i = next
		case strings.HasPrefix(line, "*** Delete File: "):
			path := normalizePatchPath(projectRoot, strings.TrimPrefix(line, "*** Delete File: "))
			delete(result, path)
			i++
		case strings.HasPrefix(line, "*** Update File: "):
			next, oldPath, newPath, updatedLines, mode, err := parseUpdateFileBlock(s, lines, i, projectRoot, result)
			if err != nil {
				return nil, err
			}
			if oldPath != newPath {
				delete(result, oldPath)
			}
			blob, err := s.WriteBlob([]byte(strings.Join(updatedLines, "\n")))
			if err != nil {
				return nil, err
			}
			result[newPath] = store.TreeEntry{Path: newPath, Blob: blob, Mode: mode}
			i = next
		default:
			return nil, fmt.Errorf("unsupported patch line %q", line)
		}
	}

	return result, nil
}

func parseAddFileBlock(lines []string, start int, projectRoot string) (int, string, []string, error) {
	path := normalizePatchPath(projectRoot, strings.TrimPrefix(lines[start], "*** Add File: "))
	var content []string
	i := start + 1
	for i < len(lines) && !startsPatchBlock(lines[i]) {
		switch {
		case strings.HasPrefix(lines[i], "+"):
			content = append(content, lines[i][1:])
		case lines[i] == "":
			content = append(content, "")
		default:
			return 0, "", nil, fmt.Errorf("unexpected line in add block for %s: %q", path, lines[i])
		}
		i++
	}
	return i, path, trimTrailingEmptyPatchLine(content), nil
}

func parseUpdateFileBlock(s *store.Store, lines []string, start int, projectRoot string, result map[string]store.TreeEntry) (int, string, string, []string, uint32, error) {
	oldPath := normalizePatchPath(projectRoot, strings.TrimPrefix(lines[start], "*** Update File: "))
	entry, ok := result[oldPath]
	if !ok {
		return 0, "", "", nil, 0, fmt.Errorf("update target %s not found in base tree", oldPath)
	}

	currentContent, mode, err := loadEntryContent(s, entry)
	if err != nil {
		return 0, "", "", nil, 0, err
	}
	baseLines := splitContentLines(string(currentContent))
	newPath := oldPath
	cursor := 0
	var out []string

	i := start + 1
	for i < len(lines) && !startsPrimaryPatchBlock(lines[i]) {
		line := lines[i]
		switch {
		case strings.HasPrefix(line, "*** Move to: "):
			newPath = normalizePatchPath(projectRoot, strings.TrimPrefix(line, "*** Move to: "))
			i++
		case strings.HasPrefix(line, "@@"):
			hunk, next, err := parseHunk(lines, i)
			if err != nil {
				return 0, "", "", nil, 0, fmt.Errorf("parse hunk for %s: %w", oldPath, err)
			}
			if hunk.oldStart < cursor {
				return 0, "", "", nil, 0, fmt.Errorf("overlapping hunk in %s", oldPath)
			}
			if hunk.oldStart > len(baseLines) {
				return 0, "", "", nil, 0, fmt.Errorf("hunk start %d beyond end of %s", hunk.oldStart, oldPath)
			}

			out = append(out, baseLines[cursor:hunk.oldStart]...)
			updatedCursor, rewritten, err := applyHunk(baseLines, hunk)
			if err != nil {
				return 0, "", "", nil, 0, fmt.Errorf("apply hunk for %s: %w", oldPath, err)
			}
			out = append(out, rewritten...)
			cursor = updatedCursor
			i = next
		case line == "*** End of File":
			i++
		case line == "":
			i++
		default:
			return 0, "", "", nil, 0, fmt.Errorf("unexpected line in update block for %s: %q", oldPath, line)
		}
	}

	out = append(out, baseLines[cursor:]...)
	return i, oldPath, newPath, out, mode, nil
}

type patchHunk struct {
	oldStart int
	oldCount int
	newStart int
	newCount int
	lines    []string
}

func parseHunk(lines []string, start int) (patchHunk, int, error) {
	m := hunkHeaderRE.FindStringSubmatch(lines[start])
	if m == nil {
		return patchHunk{}, 0, fmt.Errorf("invalid hunk header %q", lines[start])
	}

	oldStart, _ := strconv.Atoi(m[1])
	oldCount := parseOptionalCount(m[2])
	newStart, _ := strconv.Atoi(m[3])
	newCount := parseOptionalCount(m[4])

	hunk := patchHunk{
		oldStart: clampPatchStart(oldStart),
		oldCount: oldCount,
		newStart: clampPatchStart(newStart),
		newCount: newCount,
	}

	i := start + 1
	for i < len(lines) && !startsPatchBlock(lines[i]) && !strings.HasPrefix(lines[i], "@@") {
		hunk.lines = append(hunk.lines, lines[i])
		i++
	}
	return hunk, i, nil
}

func applyHunk(baseLines []string, hunk patchHunk) (int, []string, error) {
	cursor := hunk.oldStart
	var out []string
	oldSeen := 0
	newSeen := 0

	for _, line := range hunk.lines {
		switch {
		case strings.HasPrefix(line, " "):
			if cursor >= len(baseLines) {
				return 0, nil, fmt.Errorf("context line beyond end of file")
			}
			expected := line[1:]
			if baseLines[cursor] != expected {
				return 0, nil, fmt.Errorf("context mismatch: expected %q, got %q", expected, baseLines[cursor])
			}
			out = append(out, expected)
			cursor++
			oldSeen++
			newSeen++
		case strings.HasPrefix(line, "-"):
			if cursor >= len(baseLines) {
				return 0, nil, fmt.Errorf("delete line beyond end of file")
			}
			expected := line[1:]
			if baseLines[cursor] != expected {
				return 0, nil, fmt.Errorf("delete mismatch: expected %q, got %q", expected, baseLines[cursor])
			}
			cursor++
			oldSeen++
		case strings.HasPrefix(line, "+"):
			out = append(out, line[1:])
			newSeen++
		case line == "\\ No newline at end of file":
		default:
			return 0, nil, fmt.Errorf("unsupported hunk line %q", line)
		}
	}

	if oldSeen != hunk.oldCount {
		return 0, nil, fmt.Errorf("old hunk count mismatch: header=%d parsed=%d", hunk.oldCount, oldSeen)
	}
	if newSeen != hunk.newCount {
		return 0, nil, fmt.Errorf("new hunk count mismatch: header=%d parsed=%d", hunk.newCount, newSeen)
	}
	return cursor, out, nil
}

func splitPatchLines(patchText string) []string {
	normalized := strings.ReplaceAll(patchText, "\r\n", "\n")
	return strings.Split(normalized, "\n")
}

func normalizePatchPath(projectRoot, raw string) string {
	raw = filepath.ToSlash(strings.TrimSpace(raw))
	raw = strings.TrimPrefix(raw, "./")
	if filepath.IsAbs(raw) {
		if rel, err := filepath.Rel(projectRoot, raw); err == nil {
			raw = filepath.ToSlash(rel)
		}
	}
	return raw
}

func cloneEntries(base map[string]store.TreeEntry) map[string]store.TreeEntry {
	cloned := make(map[string]store.TreeEntry, len(base))
	for k, v := range base {
		cloned[k] = v
	}
	return cloned
}

func loadEntryContent(s *store.Store, entry store.TreeEntry) ([]byte, uint32, error) {
	if entry.Path == "" && entry.Blob == "" {
		return nil, 0o644, nil
	}
	content, err := s.ReadBlob(entry.Blob)
	if err != nil {
		return nil, 0, fmt.Errorf("read blob for %s: %w", entry.Path, err)
	}
	if entry.Mode == 0 {
		entry.Mode = 0o644
	}
	return content, entry.Mode, nil
}

func splitContentLines(content string) []string {
	if content == "" {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func startsPatchBlock(line string) bool {
	return startsPrimaryPatchBlock(line) || strings.HasPrefix(line, "*** Move to: ")
}

func startsPrimaryPatchBlock(line string) bool {
	return strings.HasPrefix(line, "*** Add File: ") ||
		strings.HasPrefix(line, "*** Delete File: ") ||
		strings.HasPrefix(line, "*** Update File: ") ||
		line == "*** End Patch"
}

func trimTrailingEmptyPatchLine(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	if lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

func parseOptionalCount(raw string) int {
	if raw == "" {
		return 1
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 1
	}
	return n
}

func clampPatchStart(n int) int {
	if n <= 0 {
		return 0
	}
	return n - 1
}
