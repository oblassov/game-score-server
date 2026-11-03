package engine_test

import (
	"fmt"
	"testing"

	"github.com/oblassov/game-score-server/internal/engine"
)

func TestBadPlayerInputErrMsgStatesRange(t *testing.T) {
	want := fmt.Sprintf(
		"bad value received for number of players, please try again with a number from %d to %d",
		engine.MinPlayers, engine.MaxPlayers,
	)

	if engine.BadPlayerInputErrMsg != want {
		t.Errorf("got %q, want %q", engine.BadPlayerInputErrMsg, want)
	}
}
