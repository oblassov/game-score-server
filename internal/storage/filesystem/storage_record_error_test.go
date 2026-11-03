package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oblassov/game-score-server/tests"
)

// RecordWin must return an error when the win was not persisted.
func TestRecordWinReturnsErrorWhenNotPersisted(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(path string) error
	}{
		{
			name:   "corrupt file",
			mutate: func(path string) error { return os.WriteFile(path, []byte("{bad"), 0o600) },
		},
		{
			name:   "removed file",
			mutate: os.Remove,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db.json")
			tests.AssertNoError(t, os.WriteFile(path, []byte(`[{"Name":"Cleo","Wins":10}]`), 0o600))

			store := openStore(t, path)
			tests.AssertNoError(t, tc.mutate(path))

			if err := store.RecordWin("Cleo"); err == nil {
				t.Error("RecordWin returned nil although the win was dropped")
			}
		})
	}

	t.Run("successful write returns nil", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "db.json")
		tests.AssertNoError(t, os.WriteFile(path, []byte(`[]`), 0o600))

		store := openStore(t, path)
		tests.AssertNoError(t, store.RecordWin("Cleo"))
	})
}
