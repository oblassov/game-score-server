package filesystem

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/oblassov/game-score-server/tests"
)

func TestPathNextToExecutable(t *testing.T) {
	exe, err := os.Executable()
	tests.AssertNoError(t, err)

	got, err := PathNextToExecutable("game.db.json")
	tests.AssertNoError(t, err)

	want := filepath.Join(filepath.Dir(exe), "game.db.json")
	if got != want {
		t.Errorf("got path %q, want %q", got, want)
	}
}

// The DB path must not follow the executable: under `go run` that is a temp
// build dir deleted on exit. Default to the working directory (as 003aec9 did)
// and let GAME_DB_PATH override it.
func TestDBPathFromEnv(t *testing.T) {
	cases := []struct {
		name  string
		value string
		unset bool
		want  string
	}{
		{name: "unset defaults to the working directory", unset: true, want: "game.db.json"},
		{name: "empty defaults to the working directory", value: "", want: "game.db.json"},
		{name: "blank defaults to the working directory", value: "  \t", want: "game.db.json"},
		{name: "absolute path is used as is", value: "/var/lib/poker/league.json", want: "/var/lib/poker/league.json"},
		{name: "relative path is used as is", value: "data/league.json", want: "data/league.json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GAME_DB_PATH", tc.value)
			if tc.unset {
				tests.AssertNoError(t, os.Unsetenv("GAME_DB_PATH"))
			}

			if got := DBPathFromEnv(); got != tc.want {
				t.Errorf("DBPathFromEnv() = %q, want %q", got, tc.want)
			}
		})
	}
}
