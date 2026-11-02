package filesystem

import (
	"errors"
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

// If the db file cannot be re-read under the lock, the win is dropped: the
// in-memory league and the file stay exactly as they were.
func TestRecordWinWhenDBCannotBeReRead(t *testing.T) {
	const seed = `[{"Name":"Cleo","Wins":10}]`

	cases := []struct {
		name     string
		mutate   func(path string) error
		wantFile string
		wantGone bool
		wantDir  bool
	}{
		{
			name:     "corrupt file",
			mutate:   func(path string) error { return os.WriteFile(path, []byte("{bad"), 0o600) },
			wantFile: "{bad",
		},
		{
			name:     "emptied file",
			mutate:   func(path string) error { return os.WriteFile(path, nil, 0o600) },
			wantFile: "",
		},
		{
			name:     "whitespace-only file",
			mutate:   func(path string) error { return os.WriteFile(path, []byte(" \n\t"), 0o600) },
			wantFile: " \n\t",
		},
		{
			// tape.Write also refuses a missing path, so this alone can't pin readLeague.
			name:     "removed file",
			mutate:   os.Remove,
			wantGone: true,
		},
		{
			// A directory passes tape's Stat check; only the re-read failure stops the write.
			name: "path replaced by a directory",
			mutate: func(path string) error {
				if err := os.Remove(path); err != nil {
					return err
				}

				return os.Mkdir(path, 0o700)
			},
			wantDir: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db.json")
			tests.AssertNoError(t, os.WriteFile(path, []byte(seed), 0o600))

			store := openStore(t, path)
			tests.AssertNoError(t, tc.mutate(path))

			store.RecordWin("Cleo")
			store.RecordWin("Pepper")

			tests.AssertScoreEquals(t, store.GetPlayerScore("Cleo"), 10)
			tests.AssertScoreEquals(t, store.GetPlayerScore("Pepper"), 0)

			entries, err := os.ReadDir(filepath.Dir(path))
			tests.AssertNoError(t, err)
			wantEntries := 1
			if tc.wantGone {
				wantEntries = 0
			}

			if len(entries) != wantEntries {
				t.Errorf("directory has %d entries, want %d (leftover temp file?)", len(entries), wantEntries)
			}

			if tc.wantDir {
				info, statErr := os.Stat(path)
				tests.AssertNoError(t, statErr)
				if !info.IsDir() {
					t.Error("db path was replaced by a file")
				}

				return
			}

			got, err := os.ReadFile(path)
			if tc.wantGone {
				if !errors.Is(err, os.ErrNotExist) {
					t.Errorf("db file was recreated, ReadFile err = %v", err)
				}

				return
			}

			tests.AssertNoError(t, err)
			if string(got) != tc.wantFile {
				t.Errorf("db file was touched: got %q, want %q", got, tc.wantFile)
			}
		})
	}
}
