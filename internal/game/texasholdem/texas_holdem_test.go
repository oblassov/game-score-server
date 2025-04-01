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
			{0 * time.Minute, 100},
			{10 * time.Minute, 200},
			{20 * time.Minute, 300},
			{30 * time.Minute, 400},
			{40 * time.Minute, 500},
			{50 * time.Minute, 600},
			{60 * time.Minute, 800},
			{70 * time.Minute, 1000},
			{80 * time.Minute, 2000},
			{90 * time.Minute, 4000},
			{100 * time.Minute, 8000},
		}

		checkSchedulingCases(cases, t, *blindAlerter)
	})

	t.Run("schedules alerts on game start for 7 players", func(t *testing.T) {
		blindAlerter := &tests.SpyBlindAlerter{}
		game := texasholdem.NewTexasHoldem(tests.DummyPlayerStore, blindAlerter)

		game.Start(7, io.Discard)

		cases := []tests.ScheduledAlert{
			{0 * time.Minute, 100},
			{12 * time.Minute, 200},
			{24 * time.Minute, 300},
			{36 * time.Minute, 400},
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
