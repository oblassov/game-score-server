package texasholdem_test

import (
	"fmt"
	"io"
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

func TestGame_Finish(t *testing.T) {
	playerStore := &tests.StubPlayerStore{}
	game := texasholdem.NewTexasHoldem(playerStore, tests.DummyBlindAlerter)
	winner := "Ruth"

	game.Finish(winner)
	tests.AssertPlayerWin(t, playerStore, winner)
}

func checkSchedulingCases(cases []tests.ScheduledAlert, t *testing.T, blindAlerter tests.SpyBlindAlerter) {
	t.Helper()

	for i, want := range cases {
		t.Run(fmt.Sprint(want), func(t *testing.T) {

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
