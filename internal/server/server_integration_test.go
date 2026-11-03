package server_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/internal/server"
	"github.com/oblassov/game-score-server/internal/storage/filesystem"
	"github.com/oblassov/game-score-server/tests"
)

func TestGETLeagueWithNoPlayers(t *testing.T) {
	cases := []struct {
		name string
		seed string
	}{
		{name: "empty league", seed: `[]`},
		{name: "empty file", seed: ``},
		{name: "whitespace only file", seed: "  \n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			database, cleanDatabase := tests.CreateTempFile(t, tc.seed)
			defer cleanDatabase()

			store, err := filesystem.NewPlayerStore(database)
			tests.AssertNoError(t, err)

			playerServer, _ := server.NewPlayerServer(store, tests.DummyGame)
			want := readAPIContract(t, "league-empty.json")

			response := httptest.NewRecorder()
			playerServer.ServeHTTP(response, newLeagueRequest())

			tests.AssertStatus(t, response, want.Status)
			tests.AssertContentType(t, response, "application/json")
			tests.AssertResponseBody(t, response.Body.String(), want.Body)
		})
	}
}

func TestRecordingWinsAndRetrievingThem(t *testing.T) {
	database, cleanDatabase := tests.CreateTempFile(t, `[]`)
	defer cleanDatabase()

	store, err := filesystem.NewPlayerStore(database)
	tests.AssertNoError(t, err)

	playerServer, _ := server.NewPlayerServer(store, tests.DummyGame)

	player := "Pepper"

	wantedCount := 1000
	var wg sync.WaitGroup
	wg.Add(wantedCount)

	for range wantedCount {
		go func() {
			defer wg.Done()
			playerServer.ServeHTTP(httptest.NewRecorder(), newPostWinRequest(player))
		}()
	}

	wg.Wait()

	t.Run("get score", func(t *testing.T) {
		response := httptest.NewRecorder()
		playerServer.ServeHTTP(response, newGetScoreRequest(player))

		tests.AssertStatus(t, response, http.StatusOK)
		tests.AssertResponseBody(t, response.Body.String(), "1000")
	})

	t.Run("get league", func(t *testing.T) {
		response := httptest.NewRecorder()
		playerServer.ServeHTTP(response, newLeagueRequest())
		tests.AssertStatus(t, response, http.StatusOK)

		got := getLeagueFromResponse(t, response.Body)
		want := []engine.Player{
			{Name: "Pepper", Wins: wantedCount},
		}

		tests.AssertLeague(t, got, want)
	})
}
