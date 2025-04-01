package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/oblassov/game-score-server/internal/engine"
)

const PlayerPrompt = "Please enter the number of players: "
const BadPlayerInputErrMsg = "bad value received for number of players, please try again with a number"
const BadWinnerInputErrMsg = "bad value received for winner, please try using '%NAME% wins'"

type CLI struct {
	in   *bufio.Scanner
	out  io.Writer
	game engine.Game
}

func NewCLI(in io.Reader, out io.Writer, game engine.Game) *CLI {
	return &CLI{
		in:   bufio.NewScanner(in),
		out:  out,
		game: game,
	}
}

func (cli *CLI) PlayPoker() {
	fmt.Fprint(cli.out, PlayerPrompt)
	numberOfPlayers, err := strconv.Atoi(cli.readLine())

	if err != nil {
		fmt.Fprint(cli.out, BadPlayerInputErrMsg)
		return
	}

	cli.game.Start(numberOfPlayers, cli.out)

	winnerInput := cli.readLine()
	winner, err := extractWinner(winnerInput)

	if err != nil {
		fmt.Fprint(cli.out, BadWinnerInputErrMsg)
		return
	}

	cli.game.Finish(winner)
}

func (cli *CLI) readLine() string {
	cli.in.Scan()
	return cli.in.Text()
}

func extractWinner(userInput string) (string, error) {

	if !strings.Contains(userInput, "wins") {
		return "", errors.New(BadPlayerInputErrMsg)
	}

	return strings.Replace(userInput, " wins", "", 1), nil
}
