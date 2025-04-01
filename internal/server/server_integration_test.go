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

func TestRecordingWinsAndRetrievingThem(t *testing.T) {
	database, cleanDatabase := tests.CreateTempFile(t, `[]`)
	defer cleanDatabase()

	store, err := filesystem.NewPlayerStore(database)
	tests.AssertNoError(t, err)

	server, _ := server.NewPlayerServer(store, dummyGame)

	player := "Pepper"

	wantedCount := 1000
	var wg sync.WaitGroup
	wg.Add(wantedCount)

	for range wantedCount {
		go func() {
			defer wg.Done()
			server.ServeHTTP(httptest.NewRecorder(), newPostWinRequest(player))
		}()
	}

	wg.Wait()

	t.Run("get score", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, newGetScoreRequest(player))

		tests.AssertStatus(t, response, http.StatusOK)
		tests.AssertResponseBody(t, response.Body.String(), "1000")
	})

	t.Run("get league", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, newLeagueRequest())
		tests.AssertStatus(t, response, http.StatusOK)

		got := getLeagueFromResponse(t, response.Body)
		want := []engine.Player{
			{"Pepper", wantedCount},
		}

		tests.AssertLeague(t, got, want)
	})
}
