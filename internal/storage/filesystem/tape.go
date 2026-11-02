package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

type tape struct {
	file *os.File
}

// Write atomically replaces the db file with p: it writes a temp file next to
// the db, syncs it and renames it over the original, so a failure leaves the
// db untouched.
func (t *tape) Write(p []byte) (n int, err error) {
	path := t.file.Name()

	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("couldn't stat db file %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return 0, fmt.Errorf("couldn't create temp file for %s: %w", path, err)
	}
	tmpName := tmp.Name()

	cleanup := func(cause error) (int, error) {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return 0, cause
	}

	if _, err := tmp.Write(p); err != nil {
		return cleanup(fmt.Errorf("couldn't write temp file %s: %w", tmpName, err))
	}
	if err := tmp.Sync(); err != nil {
		return cleanup(fmt.Errorf("couldn't sync temp file %s: %w", tmpName, err))
	}
	if err := tmp.Chmod(info.Mode().Perm()); err != nil {
		return cleanup(fmt.Errorf("couldn't chmod temp file %s: %w", tmpName, err))
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("couldn't close temp file %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return 0, fmt.Errorf("couldn't replace db file %s: %w", path, err)
	}

	return len(p), nil
}
