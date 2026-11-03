package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/oblassov/game-score-server/internal/engine"
)

const PlayerPrompt = "Please enter the number of players: "
const BadPlayerInputErrMsg = engine.BadPlayerInputErrMsg
const BadWinnerInputErrMsg = "bad value received for winner, please try using '%NAME% wins'"

// maxLineBytes is the longest accepted input line; a longer one is bad input.
const maxLineBytes = 64 * 1024

var errLineTooLong = errors.New("input line is too long")

type CLI struct {
	in   *bufio.Reader
	out  io.Writer
	game engine.Game
}

func NewCLI(in io.Reader, out io.Writer, game engine.Game) *CLI {
	return &CLI{
		in:   bufio.NewReader(in),
		out:  out,
		game: game,
	}
}

// PlayPoker asks for the number of players and then for the winner, asking
// again after every bad answer or failed save. It ends quietly on EOF and
// returns an error only when reading the input fails.
func (cli *CLI) PlayPoker() error {
	numberOfPlayers, err := cli.askPlayerCount()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("couldn't read the number of players: %w", err)
	}

	stop := cli.game.Start(numberOfPlayers, cli.out)
	defer stop()

	if err := cli.askWinner(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("couldn't read the winner: %w", err)
	}
	return nil
}

// askPlayerCount prompts until it gets a valid number of players.
func (cli *CLI) askPlayerCount() (int, error) {
	for {
		cli.say(PlayerPrompt)

		line, err := cli.readLine()
		if err != nil && !errors.Is(err, errLineTooLong) {
			return 0, err
		}

		numberOfPlayers, convErr := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && convErr == nil && numberOfPlayers >= engine.MinPlayers && numberOfPlayers <= engine.MaxPlayers {
			return numberOfPlayers, nil
		}

		cli.reject(BadPlayerInputErrMsg)
	}
}

// askWinner reads winner lines until the game is finished and saved.
func (cli *CLI) askWinner() error {
	for {
		line, err := cli.readLine()
		if err != nil && !errors.Is(err, errLineTooLong) {
			return err
		}

		winner, err := winnerFromLine(line, err)
		if err != nil {
			cli.reject(err.Error())
			continue
		}

		if err := cli.game.Finish(winner); err != nil {
			log.Println("couldn't finish the game: ", err)
			cli.say(engine.RecordWinFailedMsg + "\n")
			continue
		}
		return nil
	}
}

// winnerFromLine turns a winner line into a valid player name. A line that was
// too long (lineErr) is never a valid winner.
func winnerFromLine(line string, lineErr error) (string, error) {
	if lineErr != nil {
		return "", errors.New(BadWinnerInputErrMsg)
	}

	winner, err := extractWinner(line)
	if err != nil {
		return "", err
	}

	winner = engine.NormalizePlayerName(winner)
	if err := engine.ValidatePlayerName(winner); err != nil {
		return "", err
	}

	return winner, nil
}

// reject logs a bad answer and tells the user why it was refused.
func (cli *CLI) reject(msg string) {
	log.Println("bad input: ", msg)
	cli.say(msg + "\n")
}

func (cli *CLI) say(msg string) {
	if _, err := fmt.Fprint(cli.out, msg); err != nil {
		log.Println("couldn't write to the user: ", err)
	}
}

// readLine returns the next line without its line ending. It returns io.EOF
// when no more input is left, errLineTooLong (after skipping the rest of the
// line) when the line exceeds maxLineBytes, and any other read error as is.
func (cli *CLI) readLine() (string, error) {
	var line []byte
	tooLong := false

	for {
		chunk, err := cli.in.ReadSlice('\n')
		if !tooLong {
			line = append(line, chunk...)
			tooLong = len(line) > maxLineBytes
		}

		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		if err != nil && (!errors.Is(err, io.EOF) || len(line) == 0 && len(chunk) == 0) {
			return "", err
		}

		if tooLong {
			return "", errLineTooLong
		}
		return strings.TrimRight(string(line), "\r\n"), nil
	}
}

func extractWinner(userInput string) (string, error) {
	const suffix = "wins"

	if !utf8.ValidString(userInput) {
		return "", errors.New(BadWinnerInputErrMsg)
	}

	head, ok := strings.CutSuffix(strings.TrimRightFunc(userInput, unicode.IsSpace), suffix)
	if last, _ := utf8.DecodeLastRuneInString(head); !ok || !unicode.IsSpace(last) {
		return "", errors.New(BadWinnerInputErrMsg)
	}

	name := strings.TrimSpace(head)
	if name == "" {
		return "", errors.New(BadWinnerInputErrMsg)
	}

	return name, nil
}
