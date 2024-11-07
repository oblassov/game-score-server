package game

import (
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type StubPlayerStore struct {
	scores   map[string]int
	winCalls []string
	league   League
}

func (s *StubPlayerStore) GetPlayerScore(name string) int {
	score := s.scores[name]
	return score
}

func (s *StubPlayerStore) RecordWin(name string) {
	s.winCalls = append(s.winCalls, name)
}

func (p *StubPlayerStore) GetLeague() League {
	return p.league
}

type SpyBlindAlerter struct {
	alerts []struct {
		scheduledAt time.Duration
		amount      int
	}
}

func (s *SpyBlindAlerter) ScheduleAlertAt(duration time.Duration, amount int) {
	s.alerts = append(s.alerts, struct {
		scheduledAt time.Duration
		amount      int
	}{duration, amount})
}

func AssertPlayerWin(t testing.TB, store *StubPlayerStore, winner string) {
	t.Helper()

	if len(store.winCalls) != 1 {
		t.Errorf("got %d calls to RecordWin, want %d", len(store.winCalls), 1)
	}

	if store.winCalls[0] != winner {
		t.Errorf("did not store correct winner got %q, want %q", store.winCalls, winner)
	}

}

func AssertNoError(t testing.TB, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("did not expect an error, but got one, %v", err)
	}
}

func AssertLeague(t testing.TB, got, want League) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}

func AssertScoreEquals(t testing.TB, got, want int) {
	t.Helper()

	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}

}

func AssertStatus(t testing.TB, got, want int) {
	t.Helper()

	if got != want {
		t.Errorf("did not get correct status got %d, want %d", got, want)
	}

}

func AssertResponseBody(t testing.TB, got, want string) {
	t.Helper()

	if got != want {
		t.Errorf("response body is wrong, got %q, want %q", got, want)
	}

}

func AssertContentType(t testing.TB, response *httptest.ResponseRecorder, want string) {
	t.Helper()

	if response.Result().Header.Get("content-type") != want {
		t.Errorf("response did not have content-type of 'application/json', got '%v'", response.Result().Header)
	}

}
