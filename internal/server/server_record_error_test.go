package server_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/tests"
)

type failingRecordStore struct{}

func (failingRecordStore) GetPlayerScore(string) int { return 0 }

func (failingRecordStore) RecordWin(string) error { return errors.New("disk full") }

func (failingRecordStore) GetLeague() engine.League { return nil }

// A win that was not persisted must not be acknowledged with 202.
func TestPOSTPlayersReportsStoreFailure(t *testing.T) {
	playerServer := mustMakePlayerServer(t, failingRecordStore{}, tests.DummyGame)

	response := httptest.NewRecorder()
	playerServer.ServeHTTP(response, newPostWinRequest("Pepper"))

	tests.AssertStatus(t, response, http.StatusInternalServerError)
}
