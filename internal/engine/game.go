package engine

import "io"

// MinPlayers and MaxPlayers bound the accepted number of players for a game.
const (
	MinPlayers = 2
	MaxPlayers = 22
)

// BadPlayerInputErrMsg is the message sent back when the number of players
// is not a number or is outside MinPlayers..MaxPlayers.
const BadPlayerInputErrMsg = "bad value received for number of players, please try again with a number from 2 to 22"

type Game interface {
	// Start begins the game and returns a non-nil stop func that cancels any
	// pending blind alerts. Callers may call stop more than once.
	Start(numberOfPlayers int, alertDestination io.Writer) (stop func())
	Finish(winner string)
}
