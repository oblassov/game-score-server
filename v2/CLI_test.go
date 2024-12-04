package poker_test

import (
	"bytes"
	poker "game-server/v2"
	"io"
	"strings"
	"testing"
	"time"
)

var (
	dummyGame         = &poker.GameSpy{}
	dummyBlindAlerter = &poker.SpyBlindAlerter{}
	dummyPlayerStore  = &poker.StubPlayerStore{}
	dummyStdIn        = &bytes.Buffer{}
	dummyStdOut       = &bytes.Buffer{}
)

func TestCLI(t *testing.T) {

	t.Run("it prints an error when a non numeric value is entered and does not start the game", func(t *testing.T) {
		stdOut := &bytes.Buffer{}
		in := userSends("pies")
		game := &poker.GameSpy{}

		cli := poker.NewCLI(in, stdOut, game)
		cli.PlayPoker()

		assertMessagesSentToUser(t, stdOut, poker.PlayerPrompt, poker.BadPlayerInputErrMsg)
		assertGameNotStarted(t, game)
	})

	t.Run("start game with 3 players and finish game with Chris as a winner", func(t *testing.T) {
		in := userSends("3", "Chris wins")
		stdOut := &bytes.Buffer{}
		game := &poker.GameSpy{}

		cli := poker.NewCLI(in, stdOut, game)
		cli.PlayPoker()

		assertMessagesSentToUser(t, stdOut, poker.PlayerPrompt)
		assertGameStartedWith(t, game, 3)
		assertFinishCalledWith(t, game, "Chris")
	})

	t.Run("start game with 8 players and finish game with Cleo as a winner", func(t *testing.T) {
		in := userSends("8", "Cleo wins")
		stdOut := &bytes.Buffer{}
		game := &poker.GameSpy{}

		cli := poker.NewCLI(in, stdOut, game)
		cli.PlayPoker()

		assertMessagesSentToUser(t, stdOut, poker.PlayerPrompt)
		assertGameStartedWith(t, game, 8)
		assertFinishCalledWith(t, game, "Cleo")
	})

	t.Run("it prints an error when a winner is declared incorrectly", func(t *testing.T) {
		in := userSends("7", "Cleo kills")
		stdOut := &bytes.Buffer{}
		game := &poker.GameSpy{}

		cli := poker.NewCLI(in, stdOut, game)
		cli.PlayPoker()

		assertGameNotFinished(t, game)
		assertMessagesSentToUser(t, stdOut, poker.PlayerPrompt, poker.BadWinnerInputErrMsg)
	})

}

func assertFinishCalledWith(t testing.TB, game *poker.GameSpy, winner string) {
	t.Helper()

	passed := retryUntil(500*time.Millisecond, func() bool {
		return game.FinishedWith == winner
	})

	if !passed {
		t.Errorf("expected Finish called with %q, but got %q", winner, game.FinishedWith)
	}

}

func retryUntil(d time.Duration, f func() bool) bool {
	deadline := time.Now().Add(d)

	for time.Now().Before(deadline) {
		if f() {
			return true
		}
	}

	return false
}

func assertGameNotStarted(t testing.TB, game *poker.GameSpy) {
	t.Helper()

	if game.StartCalled {
		t.Errorf("game should not have started")
	}

}

func assertGameNotFinished(t testing.TB, game *poker.GameSpy) {
	t.Helper()

	if game.FinishCalled {
		t.Errorf("game should not have finished")
	}
}

func assertGameStartedWith(t testing.TB, game *poker.GameSpy, numberOfPlayers int) {
	t.Helper()

	if game.StartedWith != numberOfPlayers {
		t.Errorf("wanted Start called with %d, but go %d", numberOfPlayers, game.StartedWith)
	}

}

func assertScheduledAlert(t testing.TB, got, want poker.ScheduledAlert) {
	t.Helper()

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

}

func assertMessagesSentToUser(t testing.TB, stdOut *bytes.Buffer, messages ...string) {
	t.Helper()

	want := strings.Join(messages, "")
	got := stdOut.String()

	if got != want {
		t.Errorf("got %q sent to stdOut but expected %+v", got, messages)
	}

}

func userSends(messages ...string) io.Reader {
	return strings.NewReader(strings.Join(messages, "\n"))
}
