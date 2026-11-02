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
const BadPlayerInputErrMsg = engine.BadPlayerInputErrMsg
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

func (cli *CLI) PlayPoker() error {
	if _, err := fmt.Fprint(cli.out, PlayerPrompt); err != nil {
		log.Println("couldn't print the player number prompt: ", err)
	}
	playersInput, err := cli.readLine()
	if err != nil {
		return fmt.Errorf("couldn't read the number of players: %w", err)
	}
	numberOfPlayers, err := strconv.Atoi(playersInput)
	if err != nil || numberOfPlayers < engine.MinPlayers || numberOfPlayers > engine.MaxPlayers {
		if _, err = fmt.Fprint(cli.out, BadPlayerInputErrMsg); err != nil {
			log.Println("couldn't print the bad number of players prompt: ", err)
		}
		return nil
	}

	cli.game.Start(numberOfPlayers, cli.out)

	winnerInput, err := cli.readLine()
	if err != nil {
		return fmt.Errorf("couldn't read the winner: %w", err)
	}
	winner, err := extractWinner(winnerInput)

	if err != nil {
		if _, err := fmt.Fprint(cli.out, BadWinnerInputErrMsg); err != nil {
			log.Println("couldn't print the bad winner prompt: ", err)
		}
		return nil
	}

	cli.game.Finish(winner)
	return nil
}

// readLine returns the next line of input. A plain EOF yields an empty line
// and a nil error; only a real scanner failure is returned as an error.
func (cli *CLI) readLine() (string, error) {
	if !cli.in.Scan() {
		return "", cli.in.Err()
	}
	return cli.in.Text(), nil
}

func extractWinner(userInput string) (string, error) {
	const suffix = " wins"

	name, ok := strings.CutSuffix(userInput, suffix)
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return "", errors.New(BadWinnerInputErrMsg)
	}

	return name, nil
}
