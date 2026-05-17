package cli

import (
	"os"

	"github.com/regent-vcs/regent/internal/store"
)

func openStoreFromCWD() (*store.Store, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return store.OpenFromDir(cwd)
}
