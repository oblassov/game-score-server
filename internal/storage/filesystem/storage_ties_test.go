package filesystem

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/tests"
)

func TestGetLeagueBreaksTiesByName(t *testing.T) {
	// Enough tied players, stored in reverse name order, that an unstable
	// sort cannot return them alphabetically by accident.
	const tied = 30
	stored := make(engine.League, 0, tied+1)
	want := make(engine.League, 0, tied+1)
	want = append(want, engine.Player{Name: "Zed", Wins: 9})
	for i := tied - 1; i >= 0; i-- {
		stored = append(stored, engine.Player{Name: fmt.Sprintf("P%02d", i), Wins: 3})
	}
	stored = append(stored, engine.Player{Name: "Zed", Wins: 9})
	for i := range tied {
		want = append(want, engine.Player{Name: fmt.Sprintf("P%02d", i), Wins: 3})
	}

	data, err := json.Marshal(stored)
	tests.AssertNoError(t, err)

	database, cleanDatabase := tests.CreateTempFile(t, string(data))
	defer cleanDatabase()
	store, err := NewPlayerStore(database)
	tests.AssertNoError(t, err)

	tests.AssertLeague(t, store.GetLeague(), want)
}
