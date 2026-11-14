package state

import (
	"fmt"
	"io"
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

// WriteAtomic replaces path and syncs the file and its directory, so a forced
// reset leaves either the previous or the new content, never a truncated file.
// The temporary name is fixed, so an interrupted write leaves at most one
// stale file, which the next write replaces.
func WriteAtomic(path string, data []byte) error {
	tmpName := path + ".tmp"
	tmp, err := os.OpenFile(tmpName, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDir(filepath.Dir(path))
}

// Remove deletes path and syncs its directory. A missing file is not an error.
func Remove(path string) error {
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return syncDir(filepath.Dir(path))
}

// ReadSmall reads a state file of at most limit bytes. Larger files are an
// error, so a corrupted or planted file cannot exhaust memory.
func ReadSmall(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is larger than %d bytes", path, limit)
	}
	return data, nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
