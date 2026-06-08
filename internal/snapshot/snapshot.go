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

		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		isDir := info.IsDir()

		// On Windows, Lstat may not report ModeDir for junctions/reparse
		// points, so info.IsDir() can return false. Use os.Stat (which
		// follows links) as fallback to reliably detect directories.
		if !isDir {
			if statInfo, statErr := os.Stat(p); statErr == nil {
				if statInfo.IsDir() {
					isDir = true
				} else if info.Mode()&os.ModeSymlink != 0 {
					info = statInfo
				}
			} else if !info.Mode().IsRegular() {
				// os.Stat failed on a non-regular entry (broken symlink,
				// dangling junction, etc.). Skip to avoid ReadFile errors.
				return nil
			}
		}

		// Check ignore patterns
		if ig.Match(rel, isDir) {
			if isDir {
				return fs.SkipDir
			}
			return nil
		}

		// Skip directories (we only track files)
		if isDir {
			return nil
		}

		// Skip files larger than MaxFileSize
		if info.Size() > MaxFileSize {
			return nil
		}

		// Read file content (os.ReadFile follows symlinks automatically)
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
