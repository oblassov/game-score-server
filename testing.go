package poker

import (
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

type StubPlayerStore struct {
	Scores   map[string]int
	WinCalls []string
	League   League
}

func (s *StubPlayerStore) GetPlayerScore(name string) int {
	score := s.Scores[name]
	return score
}

func (s *StubPlayerStore) RecordWin(name string) {
	s.WinCalls = append(s.WinCalls, name)
}

func (s *StubPlayerStore) GetLeague() League {
	return s.League
}

type ScheduledAlert struct {
	At     time.Duration
	Amount int
}

func (s ScheduledAlert) String() string {
	return fmt.Sprintf("%d chips at %v", s.Amount, s.At)
}

type SpyBlindAlerter struct {
	Alerts []ScheduledAlert
}

func (s *SpyBlindAlerter) ScheduleAlertAt(at time.Duration, amount int, _ io.Writer) {
	s.Alerts = append(s.Alerts, ScheduledAlert{at, amount})
}

type GameSpy struct {
	StartedWith int
	StartCalled bool
	BlindAlert  []byte

	FinishCalled bool
	FinishedWith string
}

func (g *GameSpy) Start(numberOfPlayers int, out io.Writer) {
	g.StartCalled = true
	g.StartedWith = numberOfPlayers
	if _, err := out.Write(g.BlindAlert); err != nil {
		log.Println("couldn't write an alert: ", err)
	}
}

func (g *GameSpy) Finish(winner string) {
	g.FinishedWith = winner
}

func AssertPlayerWin(t testing.TB, store *StubPlayerStore, winner string) {
	t.Helper()

	if len(store.WinCalls) != 1 {
		t.Errorf("got %d calls to RecordWin, want %d", len(store.WinCalls), 1)
	}

	if store.WinCalls[0] != winner {
		t.Errorf("did not store correct winner got %q, want %q", store.WinCalls, winner)
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

func AssertStatus(t testing.TB, got *httptest.ResponseRecorder, want int) {
	t.Helper()

	if got.Code != want {
		t.Errorf("did not get correct status got %d, want %d", got.Code, want)
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
