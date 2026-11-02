package main

import (
	"fmt"
	"log"
	"os"

	"github.com/oblassov/game-score-server/internal/app/cli"
	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/internal/game/texasholdem"
	"github.com/oblassov/game-score-server/internal/storage/filesystem"
)

const dbFileName = "game.db.json"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run holds the program body so deferred cleanup (closing the store) runs
// before main calls log.Fatal, which would otherwise skip it.
func run() error {
	dbPath, err := filesystem.PathNextToExecutable(dbFileName)
	if err != nil {
		return err
	}

	store, closeStore, err := filesystem.PlayerStoreFromFile(dbPath)

	if err != nil {
		return err
	}
	defer func() {
		if closeErr := closeStore(); closeErr != nil {
			log.Printf("problem closing the store: %v", closeErr)
		}
	}()

	fmt.Println("Let's play poker")
	fmt.Println("Type {Name} wins to record a win")
	game := texasholdem.NewTexasHoldem(store, engine.BlindAlerterFunc(engine.Alerter))

	gameCLI := cli.NewCLI(os.Stdin, os.Stdout, game)
	return gameCLI.PlayPoker()
}
