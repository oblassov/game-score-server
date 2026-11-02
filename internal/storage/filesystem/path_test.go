package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oblassov/game-score-server/tests"
)

func TestPathNextToExecutable(t *testing.T) {
	exe, err := os.Executable()
	tests.AssertNoError(t, err)

	got, err := PathNextToExecutable("game.db.json")
	tests.AssertNoError(t, err)

	want := filepath.Join(filepath.Dir(exe), "game.db.json")
	if got != want {
		t.Errorf("got path %q, want %q", got, want)
	}
}
