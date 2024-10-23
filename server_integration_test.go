package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestRecordingWinsAndRetrievingThem(t *testing.T) {
	store := NewInMemoryPlayerStore()
	server := NewPlayerServer(store)

	player := "Pepper"

	wantedCount := 1000
	var wg sync.WaitGroup
	wg.Add(wantedCount)

	for i := 0; i < wantedCount; i++ {
		go func() {
			defer wg.Done()
			server.ServeHTTP(httptest.NewRecorder(), newPostWinRequest(player))
		}()
	}

	wg.Wait()

	t.Run("get score", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, newGetScoreRequest(player))

		assertStatus(t, response.Code, http.StatusOK)
		assertResponseBody(t, response.Body.String(), "1000")
	})

	t.Run("get league", func(t *testing.T) {
		response := httptest.NewRecorder()
		server.ServeHTTP(response, newLeagueRequest())
		assertStatus(t, response.Code, http.StatusOK)

		got := getLeagueFromResponse(t, response.Body)
		want := []Player{
			{"Pepper", 1000},
		}

		assertLeague(t, got, want)
	})
}
