package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"

	"github.com/oblassov/game-score-server/internal/app/cli"
	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/internal/game/texasholdem"
	"github.com/oblassov/game-score-server/internal/storage/filesystem"
)

const dbFileName = "game.db.json"

func main() {
	dbPath, err := filesystem.PathNextToExecutable(dbFileName)
	if err != nil {
		log.Fatal(err)
	}

	if err := run(dbPath, os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}

// run holds the program body so deferred cleanup (closing the store) runs
// before main calls log.Fatal, which would otherwise skip it.
func run(dbPath string, in io.Reader, out io.Writer) error {
	store, closeStore, err := filesystem.PlayerStoreFromFile(dbPath)

	if err != nil {
		return err
	}
	defer func() {
		if closeErr := closeStore(); closeErr != nil {
			log.Printf("problem closing the store: %v", closeErr)
		}
	}()

	// Blind alerts are written from timer goroutines, so serialise all writes
	// and drop any that arrive after the game has ended.
	guarded := &guardedWriter{w: out}
	defer guarded.close()

	if _, err := fmt.Fprintln(guarded, "Let's play poker"); err != nil {
		log.Printf("problem writing the banner: %v", err)
	}
	if _, err := fmt.Fprintln(guarded, "Type {Name} wins to record a win"); err != nil {
		log.Printf("problem writing the banner: %v", err)
	}
	game := texasholdem.NewTexasHoldem(store, engine.BlindAlerterFunc(engine.Alerter))

	gameCLI := cli.NewCLI(in, guarded, game)
	return gameCLI.PlayPoker()
}

// guardedWriter serialises writes to w and discards them once closed, so a
// late blind alert cannot write to the output after run has returned.
type guardedWriter struct {
	mu     sync.Mutex
	w      io.Writer
	closed bool
}

func (g *guardedWriter) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return len(p), nil
	}
	return g.w.Write(p)
}

func (g *guardedWriter) close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.closed = true
}
