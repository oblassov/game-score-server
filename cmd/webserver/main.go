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

const listenAddr = ":5000"

func main() {
	store, closeStore, err := filesystem.PlayerStoreFromFile(filesystem.DBPathFromEnv())
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if closeErr := closeStore(); closeErr != nil {
			log.Printf("problem closing the store: %v", closeErr)
		}
	}()

	game := texasholdem.NewTexasHoldem(store, engine.BlindAlerterFunc(engine.Alerter))
	playerServer, err := server.NewPlayerServer(store, game)

	if err != nil {
		log.Printf("problem creating player server %v", err)
		return
	}

	srv := newHTTPServer(playerServer.Handler)

	log.Printf("Starting a server on localhost%s", listenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("could not listen on %s %v", listenAddr, err)
		return
	}
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         listenAddr,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  5 * time.Second,
	}
}
