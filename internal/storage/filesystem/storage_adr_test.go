package filesystem

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/tests"
)

// ADR 0002: the store trims names itself so no caller can skip it.
func TestStoresTrimPlayerNames(t *testing.T) {
	t.Run("padded name is recorded trimmed", func(t *testing.T) {
		store := openStore(t, filepath.Join(t.TempDir(), "db.json"))
		tests.AssertNoError(t, store.RecordWin(" Alice "))

		tests.AssertLeague(t, store.GetLeague(), engine.League{{Name: "Alice", Wins: 1}})
	})

	t.Run("padded lookup finds the trimmed name", func(t *testing.T) {
		store := openStore(t, filepath.Join(t.TempDir(), "db.json"))
		tests.AssertNoError(t, store.RecordWin("Alice"))

		tests.AssertScoreEquals(t, store.GetPlayerScore(" Alice "), 1)
	})

	t.Run("padded and plain wins share one row", func(t *testing.T) {
		store := openStore(t, filepath.Join(t.TempDir(), "db.json"))
		for _, name := range []string{"Alice", "Alice", "\tAlice\n"} {
			tests.AssertNoError(t, store.RecordWin(name))
		}

		tests.AssertScoreEquals(t, store.GetPlayerScore("Alice"), 3)
		if got := store.GetLeague(); len(got) != 1 {
			t.Errorf("got %d league rows, want 1: %v", len(got), got)
		}
	})
}

func TestPaddedNamesInDBMergeOnLoad(t *testing.T) {
	const seed = `[{"Name":" Alice","Wins":2},{"Name":"Alice","Wins":3},{"Name":"  Bob ","Wins":1}]`

	path := filepath.Join(t.TempDir(), "db.json")
	tests.AssertNoError(t, os.WriteFile(path, []byte(seed), 0o600))

	store := openStore(t, path)

	t.Run("loaded league", func(t *testing.T) {
		tests.AssertScoreEquals(t, store.GetPlayerScore("Alice"), 5)
		tests.AssertScoreEquals(t, store.GetPlayerScore("Bob"), 1)
		tests.AssertLeague(t, store.GetLeague(), engine.League{{Name: "Alice", Wins: 5}, {Name: "Bob", Wins: 1}})
	})

	t.Run("win after load", func(t *testing.T) {
		tests.AssertNoError(t, store.RecordWin("Alice"))
		tests.AssertScoreEquals(t, store.GetPlayerScore("Alice"), 6)

		reopened := openStore(t, path)
		tests.AssertLeague(t, reopened.GetLeague(), engine.League{{Name: "Alice", Wins: 6}, {Name: "Bob", Wins: 1}})
	})
}

func TestNullDBIsEmptyLeague(t *testing.T) {
	cases := []struct {
		name string
		seed string
	}{
		{name: "null", seed: "null"},
		{name: "padded null", seed: " null \n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "db.json")
			tests.AssertNoError(t, os.WriteFile(path, []byte(tc.seed), 0o600))

			store := openStore(t, path)

			league := store.GetLeague()
			if league == nil {
				t.Error("got a nil league, want a non-nil empty one")
			}
			got, err := json.Marshal(league)
			tests.AssertNoError(t, err)
			if string(got) != "[]" {
				t.Errorf("got %s, want []", got)
			}

			tests.AssertNoError(t, store.RecordWin("Alice"))
			tests.AssertScoreEquals(t, store.GetPlayerScore("Alice"), 1)
		})
	}
}

func TestCloseTwiceReturnsNil(t *testing.T) {
	store := openStore(t, filepath.Join(t.TempDir(), "db.json"))

	for i := 1; i <= 2; i++ {
		if err := store.Close(); err != nil {
			t.Errorf("Close call %d: got %v, want nil", i, err)
		}
	}
}

func TestLockFileModeMatchesDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")

	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	tests.AssertNoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	store, err := NewPlayerStore(file)
	tests.AssertNoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	dbInfo, err := os.Stat(path)
	tests.AssertNoError(t, err)
	lockInfo, err := os.Stat(path + ".lock")
	tests.AssertNoError(t, err)

	if got, want := lockInfo.Mode().Perm(), dbInfo.Mode().Perm(); got != want {
		t.Errorf("lock file mode %v, want the db file's %v", got, want)
	}
}

func TestFailedReReadIsLogged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db.json")
	tests.AssertNoError(t, os.WriteFile(path, []byte(`[{"Name":"Alice","Wins":1}]`), 0o600))

	store := openStore(t, path)
	tests.AssertNoError(t, os.WriteFile(path, []byte("not json"), 0o600))

	var logged bytes.Buffer
	prevOutput, prevFlags := log.Writer(), log.Flags()
	log.SetOutput(&logged)
	t.Cleanup(func() {
		log.SetOutput(prevOutput)
		log.SetFlags(prevFlags)
	})

	tests.AssertLeague(t, store.GetLeague(), engine.League{{Name: "Alice", Wins: 1}})

	if !strings.Contains(logged.String(), path) {
		t.Errorf("log %q does not mention the db file %s", logged.String(), path)
	}
}
