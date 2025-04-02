package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
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
	if _, err := fmt.Fprint(cli.out, PlayerPrompt); err != nil {
		log.Println("couldn't print the player number prompt: ", err)
	}
	numberOfPlayers, err := strconv.Atoi(cli.readLine())
	if err != nil {
		if _, err = fmt.Fprint(cli.out, BadPlayerInputErrMsg); err != nil {
			log.Println("couldn't print the bad number of players prompt: ", err)
		}
		return
	}

	cli.game.Start(numberOfPlayers, cli.out)

	winnerInput := cli.readLine()
	winner, err := extractWinner(winnerInput)

	if err != nil {
		if _, err := fmt.Fprint(cli.out, BadWinnerInputErrMsg); err != nil {
			log.Println("couldn't print the bad winner prompt: ", err)
		}
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
