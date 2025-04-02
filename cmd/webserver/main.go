package main

import (
	"log"
	"net/http"
	"time"

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
	playerServer, err := server.NewPlayerServer(store, game)

	if err != nil {
		log.Printf("problem creating player server %v", err)
		return
	}

	server := &http.Server{
		Addr:         "5000",
		Handler:      playerServer.Handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  5 * time.Second,
	}

	log.Println("Starting a server on localhost:5000")
	if err := server.ListenAndServe(); err != nil {
		log.Printf("could not listen on port 5000 %v", err)
		return
	}
}
