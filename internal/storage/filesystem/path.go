package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
)

// PathNextToExecutable returns the path of a file called name located in the
// same directory as the running executable. Symlinks are not resolved.
func PathNextToExecutable(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exe), name), nil
}
