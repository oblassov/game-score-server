package filesystem

import (
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

// Currently exits the test binary via log.Fatalf instead of returning an error.
func TestPlayerStoreFromFileReturnsErrorOnBadPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "db.json")

	_, closeFunc, err := PlayerStoreFromFile(path)
	if err == nil {
		closeFunc()
		t.Fatal("expected an error for an unopenable path, got nil")
	}
}
