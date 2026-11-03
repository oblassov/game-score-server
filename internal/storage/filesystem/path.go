package filesystem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultDBFileName is the league DB path used when GAME_DB_PATH is not set.
// It is relative, so it is resolved against the working directory.
const DefaultDBFileName = "game.db.json"

// DBPathFromEnv returns the league DB path: the GAME_DB_PATH environment
// variable when it is set and not blank, otherwise DefaultDBFileName, which is
// resolved against the working directory.
func DBPathFromEnv() string {
	if path := os.Getenv("GAME_DB_PATH"); strings.TrimSpace(path) != "" {
		return path
	}

	return DefaultDBFileName
}

// PathNextToExecutable returns the path of a file called name located in the
// same directory as the running executable. Symlinks are not resolved.
func PathNextToExecutable(name string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("finding executable path: %w", err)
	}

	return filepath.Join(filepath.Dir(exe), name), nil
}
