package filesystem

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/oblassov/game-score-server/tests"
)

// Fails only under -race: GetLeague sorts the shared slice under RLock.
func TestGetLeagueConcurrentReads(t *testing.T) {
	database, cleanDatabase := tests.CreateTempFile(t, `[
		{"Name": "Cleo", "Wins": 10},
		{"Name": "Chris", "Wins": 33}]`)
	defer cleanDatabase()

	store, err := NewPlayerStore(database)
	tests.AssertNoError(t, err)

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			store.GetLeague()
		}()
	}
	wg.Wait()
}

func TestNewPlayerStoreWrapsUnderlyingErrors(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*os.File) error
		matches func(error) bool
	}{
		{
			name: "corrupt JSON keeps the decode error",
			prepare: func(f *os.File) error {
				_, err := f.WriteString("{not json")
				return err
			},
			matches: func(err error) bool {
				var syntaxErr *json.SyntaxError
				return errors.As(err, &syntaxErr)
			},
		},
		{
			name:    "closed file keeps os.ErrClosed",
			prepare: func(f *os.File) error { return f.Close() },
			matches: func(err error) bool { return errors.Is(err, os.ErrClosed) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database, err := os.CreateTemp(t.TempDir(), "db")
			if err != nil {
				t.Fatalf("could not create temp file: %v", err)
			}
			defer func() { _ = database.Close() }()

			if prepErr := tc.prepare(database); prepErr != nil {
				t.Fatalf("could not prepare file: %v", prepErr)
			}

			_, err = NewPlayerStore(database)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}

			if !tc.matches(err) {
				t.Errorf("underlying error lost from chain: %v", err)
			}
		})
	}
}

// Currently exits the test binary via log.Fatalf instead of returning an error.
func TestPlayerStoreFromFileReturnsErrorOnBadPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "db.json")

	_, closeFunc, err := PlayerStoreFromFile(path)
	if err == nil {
		closeFunc()
		t.Fatal("expected an error for an unopenable path, got nil")
	}
}
