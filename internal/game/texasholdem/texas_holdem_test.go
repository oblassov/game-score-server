package texasholdem_test

import (
	"bytes"
	"errors"
	"io"
	"log"
	"strings"
	"testing"
	"time"

	"github.com/oblassov/game-score-server/internal/game/texasholdem"
	"github.com/oblassov/game-score-server/tests"
)

func TestGame_Start(t *testing.T) {
	t.Run("schedules alerts on game start for 5 players", func(t *testing.T) {
		blindAlerter := &tests.SpyBlindAlerter{}
		game := texasholdem.NewTexasHoldem(tests.DummyPlayerStore, blindAlerter)

		game.Start(5, io.Discard)

		cases := []tests.ScheduledAlert{
			{At: 0 * time.Minute, Amount: 100},
			{At: 10 * time.Minute, Amount: 200},
			{At: 20 * time.Minute, Amount: 300},
			{At: 30 * time.Minute, Amount: 400},
			{At: 40 * time.Minute, Amount: 500},
			{At: 50 * time.Minute, Amount: 600},
			{At: 60 * time.Minute, Amount: 800},
			{At: 70 * time.Minute, Amount: 1000},
			{At: 80 * time.Minute, Amount: 2000},
			{At: 90 * time.Minute, Amount: 4000},
			{At: 100 * time.Minute, Amount: 8000},
		}

		checkSchedulingCases(cases, t, *blindAlerter)
	})

	t.Run("schedules alerts on game start for 7 players", func(t *testing.T) {
		blindAlerter := &tests.SpyBlindAlerter{}
		game := texasholdem.NewTexasHoldem(tests.DummyPlayerStore, blindAlerter)

		game.Start(7, io.Discard)

		cases := []tests.ScheduledAlert{
			{At: 0 * time.Minute, Amount: 100},
			{At: 12 * time.Minute, Amount: 200},
			{At: 24 * time.Minute, Amount: 300},
			{At: 36 * time.Minute, Amount: 400},
		}

		checkSchedulingCases(cases, t, *blindAlerter)
	})
}

func TestGame_StartReturnsStopThatCancelsAllAlerts(t *testing.T) {
	blindAlerter := &tests.SpyBlindAlerter{}
	game := texasholdem.NewTexasHoldem(tests.DummyPlayerStore, blindAlerter)

	stop := game.Start(5, io.Discard)

	if blindAlerter.Cancelled != 0 {
		t.Fatalf("%d alerts cancelled before stop was called, want 0", blindAlerter.Cancelled)
	}

	stop()

	if got, want := blindAlerter.Cancelled, len(blindAlerter.Alerts); got != want {
		t.Errorf("stop cancelled %d alerts, want all %d scheduled", got, want)
	}
}

// Rewritten for ADR 0002: Finish now returns the RecordWin error.
func TestGame_Finish(t *testing.T) {
	playerStore := &tests.StubPlayerStore{}
	game := texasholdem.NewTexasHoldem(playerStore, tests.DummyBlindAlerter)
	winner := "Ruth"

	if err := game.Finish(winner); err != nil {
		t.Errorf("Finish returned %v, want nil", err)
	}

	tests.AssertPlayerWin(t, playerStore, winner)
}

type failingRecordStore struct {
	tests.StubPlayerStore
	err error
}

func (s *failingRecordStore) RecordWin(string) error { return s.err }

func TestGame_FinishReturnsAndLogsRecordWinError(t *testing.T) {
	var logs bytes.Buffer

	prevOut := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(prevOut) })

	storeErr := errors.New("disk full")
	game := texasholdem.NewTexasHoldem(&failingRecordStore{err: storeErr}, tests.DummyBlindAlerter)
	winner := "Ruth"

	err := game.Finish(winner)

	if !errors.Is(err, storeErr) {
		t.Errorf("Finish returned %v, want an error wrapping %v", err, storeErr)
	}

	if !strings.Contains(logs.String(), winner) {
		t.Errorf("log %q does not contain the player name %q", logs.String(), winner)
	}
}

func checkSchedulingCases(cases []tests.ScheduledAlert, t *testing.T, blindAlerter tests.SpyBlindAlerter) {
	t.Helper()

	for i, want := range cases {
		t.Run(want.String(), func(t *testing.T) {

			if len(blindAlerter.Alerts) <= i {
				t.Fatalf("alert %d was not scheduled %v", i, blindAlerter.Alerts)
			}

			got := blindAlerter.Alerts[i]
			assertScheduledAlert(t, got, want)
		})
	}
}

func assertScheduledAlert(t testing.TB, got, want tests.ScheduledAlert) {
	t.Helper()

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
