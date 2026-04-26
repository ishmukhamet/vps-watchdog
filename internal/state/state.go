package state

import (
	"fmt"
	"os"
	"path/filepath"
)

const DefaultDir = "/var/lib/vps-watchdog"

type Store struct {
	Dir string
}

func New(dir string) Store {
	if dir == "" {
		dir = DefaultDir
	}
	return Store{Dir: dir}
}

func (s Store) Ensure() error {
	if err := os.MkdirAll(s.Dir, 0o750); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	return nil
}

func (s Store) Path(name string) string {
	return filepath.Join(s.Dir, name)
}
