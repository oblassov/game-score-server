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

const dbFileName = "./game.db.json"

func main() {
	store, closeStore, err := filesystem.PlayerStoreFromFile(dbFileName)

	if err != nil {
		log.Fatal(err)
	}
	defer closeStore()

	fmt.Println("Let's play poker")
	fmt.Println("Type {Name} wins to record a win")
	game := texasholdem.NewTexasHoldem(store, engine.BlindAlerterFunc(engine.Alerter))

	gameCLI := cli.NewCLI(os.Stdin, os.Stdout, game)
	gameCLI.PlayPoker()
}
