package snapshot

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/regent-vcs/regent/internal/ignore"
	"github.com/regent-vcs/regent/internal/store"
)

// MaxFileSize is the maximum file size to snapshot (10 MB by default)
const MaxFileSize = 10 * 1024 * 1024

// Snapshot walks the workspace and creates a tree object
func Snapshot(s *store.Store, root string, ig *ignore.Matcher) (store.Hash, error) {
	var entries []store.TreeEntry

	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, _ := filepath.Rel(root, p)
		if rel == "." {
			return nil
		}

		// Check ignore patterns
		if ig.Match(rel, d.IsDir()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Skip directories (we only track files)
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Skip symlinks for v0 (simplifies semantics)
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Skip files larger than MaxFileSize
		if info.Size() > MaxFileSize {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(p)
		if err != nil {
			return err
		}

		// Write blob
		h, err := s.WriteBlob(content)
		if err != nil {
			return err
		}

		// Use forward slashes for cross-platform consistency
		entries = append(entries, store.TreeEntry{
			Path: filepath.ToSlash(rel),
			Blob: h,
			Mode: uint32(info.Mode().Perm()),
		})

		return nil
	})

	if err != nil {
		return "", err
	}

	tree := &store.Tree{Entries: entries}
	return s.WriteTree(tree)
}
