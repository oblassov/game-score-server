package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oblassov/game-score-server/tests"
)

func openStore(t *testing.T, path string) *PlayerStore {
	t.Helper()

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	tests.AssertNoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	store, err := NewPlayerStore(file)
	tests.AssertNoError(t, err)

	return store
}

// Two stores (two processes) on one DB file must not overwrite each other's wins.
func TestRecordWinKeepsWinsRecordedByAnotherStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")

	first := openStore(t, path)
	second := openStore(t, path)

	first.RecordWin("Alice")
	second.RecordWin("Bob")
	second.RecordWin("Alice")

	reopened := openStore(t, path)

	tests.AssertScoreEquals(t, reopened.GetPlayerScore("Alice"), 2)
	tests.AssertScoreEquals(t, reopened.GetPlayerScore("Bob"), 1)
}
