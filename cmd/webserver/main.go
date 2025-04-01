package main

import (
	"log"
	"net/http"

	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/internal/game/texasholdem"
	"github.com/oblassov/game-score-server/internal/server"
	"github.com/oblassov/game-score-server/internal/storage/filesystem"
)

const dbFileName = "./game.db.json"

func main() {

	store, closeStore, err := filesystem.PlayerStoreFromFile(dbFileName)

	if err != nil {
		log.Fatal(err)
	}
	defer closeStore()

	game := texasholdem.NewTexasHoldem(store, engine.BlindAlerterFunc(engine.Alerter))
	server, err := server.NewPlayerServer(store, game)

	if err != nil {
		log.Fatalf("problem creating player server %v", err)
	}

	if err := http.ListenAndServe(":5000", server); err != nil {
		log.Fatalf("could not listen on port 5000 %v", err)
	}
}
