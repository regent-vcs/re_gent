package codex

import "github.com/regent-vcs/regent/internal/store"

func computeAndWriteBlameForStep(s *store.Store, parentHash, currentStepHash, treeHash store.Hash) error {
	tree, err := s.ReadTree(treeHash)
	if err != nil {
		return err
	}

	var parentTree *store.Tree
	if parentHash != "" {
		parentStep, err := s.ReadStep(parentHash)
		if err == nil {
			parentTree, _ = s.ReadTree(parentStep.Tree)
		}
	}

	for _, entry := range tree.Entries {
		var parentEntry *store.TreeEntry
		if parentTree != nil {
			for i := range parentTree.Entries {
				if parentTree.Entries[i].Path == entry.Path {
					parentEntry = &parentTree.Entries[i]
					break
				}
			}
		}

		if parentEntry != nil && parentEntry.Blob == entry.Blob {
			oldBlame, err := s.ReadBlameForFile(parentHash, entry.Path)
			if err == nil {
				if err := s.WriteBlameForFile(currentStepHash, entry.Path, oldBlame); err != nil {
					return err
				}
				continue
			}
		}

		var oldContent []byte
		var oldBlame *store.BlameMap
		if parentEntry != nil {
			oldContent, _ = s.ReadBlob(parentEntry.Blob)
			oldBlame, _ = s.ReadBlameForFile(parentHash, parentEntry.Path)
		}

		newContent, err := s.ReadBlob(entry.Blob)
		if err != nil {
			return err
		}

		newBlame := store.ComputeBlame(oldContent, newContent, oldBlame, currentStepHash)
		if err := s.WriteBlameForFile(currentStepHash, entry.Path, newBlame); err != nil {
			return err
		}
	}

	return nil
}
