package engine

import "io"

// MinPlayers and MaxPlayers bound the accepted number of players for a game.
const (
	MinPlayers = 1
	MaxPlayers = 100
)

// BadPlayerInputErrMsg is the message sent back when the number of players
// is not a number or is outside MinPlayers..MaxPlayers.
const BadPlayerInputErrMsg = "bad value received for number of players, please try again with a number"

type Game interface {
	Start(numberOfPlayers int, alertDestination io.Writer)
	Finish(winner string)
}
