package cli_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/oblassov/game-score-server/internal/app/cli"
	"github.com/oblassov/game-score-server/tests"
)

func TestCLI(t *testing.T) {

	t.Run("it prints an error when a non numeric value is entered and does not start the game", func(t *testing.T) {
		stdOut := &bytes.Buffer{}
		in := userSends("pies")
		game := &tests.GameSpy{}

		cliApp := cli.NewCLI(in, stdOut, game)
		cliApp.PlayPoker()

		assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt, cli.BadPlayerInputErrMsg)
		assertGameNotStarted(t, game)
	})

	for _, count := range []string{"0", "-1", "-100", "101", "1000000", "9223372036854775807"} {
		t.Run("it rejects the out of range player count "+count+" and does not start the game", func(t *testing.T) {
			stdOut := &bytes.Buffer{}
			in := userSends(count, "Chris wins")
			game := &tests.GameSpy{}

			cliApp := cli.NewCLI(in, stdOut, game)
			cliApp.PlayPoker()

			assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt, cli.BadPlayerInputErrMsg)
			assertGameNotStarted(t, game)
			assertGameNotFinished(t, game)
		})
	}

	for _, count := range []struct {
		input string
		want  int
	}{{"1", 1}, {"100", 100}} {
		t.Run("it accepts the boundary player count "+count.input, func(t *testing.T) {
			stdOut := &bytes.Buffer{}
			in := userSends(count.input, "Chris wins")
			game := &tests.GameSpy{}

			cliApp := cli.NewCLI(in, stdOut, game)
			cliApp.PlayPoker()

			assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt)
			assertGameStartedWith(t, game, count.want)
			assertFinishCalledWith(t, game, "Chris")
		})
	}

	t.Run("start game with 3 players and finish game with Chris as a winner", func(t *testing.T) {
		in := userSends("3", "Chris wins")
		stdOut := &bytes.Buffer{}
		game := &tests.GameSpy{}

		cliApp := cli.NewCLI(in, stdOut, game)
		cliApp.PlayPoker()

		assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt)
		assertGameStartedWith(t, game, 3)
		assertFinishCalledWith(t, game, "Chris")
	})

	t.Run("start game with 8 players and finish game with Cleo as a winner", func(t *testing.T) {
		in := userSends("8", "Cleo wins")
		stdOut := &bytes.Buffer{}
		game := &tests.GameSpy{}

		cliApp := cli.NewCLI(in, stdOut, game)
		cliApp.PlayPoker()

		assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt)
		assertGameStartedWith(t, game, 8)
		assertFinishCalledWith(t, game, "Cleo")
	})

	t.Run("it prints an error when a winner is declared incorrectly", func(t *testing.T) {
		in := userSends("7", "Cleo kills")
		stdOut := &bytes.Buffer{}
		game := &tests.GameSpy{}

		cliApp := cli.NewCLI(in, stdOut, game)
		cliApp.PlayPoker()

		assertGameNotFinished(t, game)
		assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt, cli.BadWinnerInputErrMsg)
	})

}

func TestCLIPlayerCountTrimsWhitespace(t *testing.T) {
	for name, sent := range map[string]string{
		"trailing space":    "4 ",
		"leading space":     " 4",
		"tabs":              "\t4\t",
		"carriage return":   "4\r",
		"spaces both sides": "  4  ",
	} {
		t.Run(name, func(t *testing.T) {
			stdOut := &bytes.Buffer{}
			game := &tests.GameSpy{}

			cli.NewCLI(userSends(sent, "Chris wins"), stdOut, game).PlayPoker()

			assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt)
			assertGameStartedWith(t, game, 4)
			assertFinishCalledWith(t, game, "Chris")
		})
	}
}

func TestCLIRejectsInvalidUTF8Winner(t *testing.T) {
	for name, sent := range map[string]string{
		"only an invalid byte":          "\xff wins",
		"invalid byte inside the name":  "Al\xffice wins",
		"truncated multibyte character": "Al\xe2\x82 wins",
	} {
		t.Run(name, func(t *testing.T) {
			stdOut := &bytes.Buffer{}
			game := &tests.GameSpy{}

			cli.NewCLI(userSends("3", sent), stdOut, game).PlayPoker()

			assertGameNotFinished(t, game)
			assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt, cli.BadWinnerInputErrMsg)
		})
	}
}

func assertFinishCalledWith(t testing.TB, game *tests.GameSpy, winner string) {
	t.Helper()

	passed := retryUntil(500*time.Millisecond, func() bool {
		return game.GetFinishedWith() == winner
	})

	if !passed {
		t.Errorf("expected Finish called with %q, but got %q", winner, game.GetFinishedWith())
	}

}

func retryUntil(d time.Duration, f func() bool) bool {
	deadline := time.Now().Add(d)

	for time.Now().Before(deadline) {
		if f() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}

	return false
}

func assertGameNotStarted(t testing.TB, game *tests.GameSpy) {
	t.Helper()

	if game.GetStartCalled() {
		t.Errorf("game should not have started")
	}

}

func assertGameNotFinished(t testing.TB, game *tests.GameSpy) {
	t.Helper()

	if game.GetFinishCalled() {
		t.Errorf("game should not have finished")
	}
}

func assertGameStartedWith(t testing.TB, game *tests.GameSpy, numberOfPlayers int) {
	t.Helper()

	if game.GetStartedWith() != numberOfPlayers {
		t.Errorf("wanted Start called with %d, but go %d", numberOfPlayers, game.GetStartedWith())
	}

}

type failingReader struct {
	data string
	err  error
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.data == "" {
		return 0, r.err
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}

func TestCLIReadErrors(t *testing.T) {
	errBoom := errors.New("boom")

	t.Run("a read error on the player count is returned and not reported as bad input", func(t *testing.T) {
		stdOut := &bytes.Buffer{}
		game := &tests.GameSpy{}

		err := cli.NewCLI(&failingReader{err: errBoom}, stdOut, game).PlayPoker()

		if !errors.Is(err, errBoom) {
			t.Fatalf("got error %v, want it to wrap %v", err, errBoom)
		}
		assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt)
		assertGameNotStarted(t, game)
	})

	t.Run("a read error on the winner line is returned and the game is not finished", func(t *testing.T) {
		stdOut := &bytes.Buffer{}
		game := &tests.GameSpy{}
		in := &failingReader{data: "3\n", err: errBoom}

		err := cli.NewCLI(in, stdOut, game).PlayPoker()

		if !errors.Is(err, errBoom) {
			t.Fatalf("got error %v, want it to wrap %v", err, errBoom)
		}
		assertGameNotFinished(t, game)
		if strings.Contains(stdOut.String(), cli.BadWinnerInputErrMsg) {
			t.Errorf("read error was reported as bad winner input: %q", stdOut.String())
		}
	})

	t.Run("plain EOF is still reported as bad input without an error", func(t *testing.T) {
		stdOut := &bytes.Buffer{}
		game := &tests.GameSpy{}

		err := cli.NewCLI(userSends(), stdOut, game).PlayPoker()

		tests.AssertNoError(t, err)
		assertMessagesSentToUser(t, stdOut, cli.PlayerPrompt, cli.BadPlayerInputErrMsg)
	})
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
