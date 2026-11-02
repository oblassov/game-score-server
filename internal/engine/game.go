package engine

import "io"

// MinPlayers and MaxPlayers bound the accepted number of players for a game.
const (
	MinPlayers = 1
	MaxPlayers = 100
)

type Game interface {
	Start(numberOfPlayers int, alertDestination io.Writer)
	Finish(winner string)
}
