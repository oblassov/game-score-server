package filesystem

import (
	"testing"

	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/tests"
)

func TestFileSystemStore(t *testing.T) {
	t.Run("league from a reader and seeking", func(t *testing.T) {

		database, cleanDatabase := tests.CreateTempFile(t, `[
		{"Name": "Cleo", "Wins": 10},
		{"Name": "Chris", "Wins": 33}]`)
		defer cleanDatabase()

		store, err := NewPlayerStore(database)

		tests.AssertNoError(t, err)

		got := store.GetLeague()
		want := []engine.Player{
			{Name: "Chris", Wins: 33},
			{Name: "Cleo", Wins: 10},
		}

		tests.AssertLeague(t, got, want)

		// read again
		got = store.GetLeague()
		tests.AssertLeague(t, got, want)
	})

	t.Run("get player score", func(t *testing.T) {

		database, cleanDatabase := tests.CreateTempFile(t, `[
		{"Name": "Cleo", "Wins": 10},
		{"Name": "Chris", "Wins": 33}]`)
		defer cleanDatabase()

		store, err := NewPlayerStore(database)

		tests.AssertNoError(t, err)

		got := store.GetPlayerScore("Chris")
		want := 33

		tests.AssertScoreEquals(t, got, want)

	})

	t.Run("store wins for existing players", func(t *testing.T) {

		database, cleanDatabase := tests.CreateTempFile(t, `[
		{"Name": "Cleo", "Wins": 10},
		{"Name": "Chris", "Wins": 33}]`)
		defer cleanDatabase()

		store, err := NewPlayerStore(database)
		store.RecordWin("Chris")

		tests.AssertNoError(t, err)

		got := store.GetPlayerScore("Chris")
		want := 34

		tests.AssertScoreEquals(t, got, want)

	})

	t.Run("store wins for new players", func(t *testing.T) {
		database, cleanDatabase := tests.CreateTempFile(t, `[
		{"Name": "Cleo", "Wins": 10},
		{"Name": "Chris", "Wins": 33}]`)
		defer cleanDatabase()

		store, err := NewPlayerStore(database)
		store.RecordWin("Pepper")

		tests.AssertNoError(t, err)

		got := store.GetPlayerScore("Pepper")
		want := 1
		tests.AssertScoreEquals(t, got, want)
	})

	t.Run("works with an empty file", func(t *testing.T) {
		database, cleanDatabase := tests.CreateTempFile(t, "")
		defer cleanDatabase()

		_, err := NewPlayerStore(database)

		tests.AssertNoError(t, err)
	})

	t.Run("league sorted", func(t *testing.T) {
		database, cleanDatabase := tests.CreateTempFile(t, `[
			{"Name": "Cleo", "Wins": 10},
			{"Name": "Chris", "Wins": 33}]`)
		defer cleanDatabase()
		store, err := NewPlayerStore(database)

		tests.AssertNoError(t, err)

		got := store.GetLeague()

		want := engine.League{
			{Name: "Chris", Wins: 33},
			{Name: "Cleo", Wins: 10},
		}

		tests.AssertLeague(t, got, want)

		// read again
		got = store.GetLeague()
		tests.AssertLeague(t, got, want)
	})
}
