//go:build unix

package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/tests"
)

// holdLock takes the db's sidecar flock exclusively, as a hung process would,
// until the test ends.
func holdLock(t *testing.T, dbPath string) {
	t.Helper()

	lockFile, err := os.OpenFile(dbPath+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	tests.AssertNoError(t, err)
	t.Cleanup(func() { _ = lockFile.Close() })

	tests.AssertNoError(t, syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX))
	t.Cleanup(func() { _ = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN) })
}

// returnsWithin runs call in a goroutine and fails the test if it hasn't
// returned after the ADR 0004 timeout plus a margin.
func returnsWithin[T any](t *testing.T, call func() T) T {
	t.Helper()

	done := make(chan T, 1)
	go func() { done <- call() }()

	select {
	case result := <-done:
		return result
	case <-time.After(8 * time.Second):
		t.Fatal("blocked on the db lock")
		var zero T

		return zero
	}
}

func TestLockWaitIsBounded(t *testing.T) {
	t.Parallel()

	const seed = `[{"Name":"Alice","Wins":1}]`

	t.Run("RecordWin returns ErrLockTimeout while the lock is held", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "db.json")
		tests.AssertNoError(t, os.WriteFile(path, []byte(seed), 0o600))
		store := openStore(t, path)
		holdLock(t, path)

		err := returnsWithin(t, func() error { return store.RecordWin("Bob") })

		if !errors.Is(err, ErrLockTimeout) {
			t.Errorf("got %v, want an error wrapping ErrLockTimeout", err)
		}
	})

	t.Run("GetLeague serves the cached league while the lock is held", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "db.json")
		tests.AssertNoError(t, os.WriteFile(path, []byte(seed), 0o600))
		store := openStore(t, path)
		holdLock(t, path)

		got := returnsWithin(t, store.GetLeague)

		tests.AssertLeague(t, got, engine.League{{Name: "Alice", Wins: 1}})
	})

	t.Run("NewPlayerStore returns ErrLockTimeout and leaves a blank db unseeded", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "db.json")
		file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
		tests.AssertNoError(t, err)
		t.Cleanup(func() { _ = file.Close() })
		holdLock(t, path)

		type opened struct {
			store *PlayerStore
			err   error
		}
		result := returnsWithin(t, func() opened {
			store, openErr := NewPlayerStore(file)

			return opened{store: store, err: openErr}
		})
		if result.store != nil {
			t.Cleanup(func() { _ = result.store.Close() })
		}

		if !errors.Is(result.err, ErrLockTimeout) {
			t.Errorf("got %v, want an error wrapping ErrLockTimeout", result.err)
		}

		info, err := os.Stat(path)
		tests.AssertNoError(t, err)
		if info.Size() != 0 {
			t.Errorf("db file was seeded while the lock was held: size %d, want 0", info.Size())
		}
	})
}
