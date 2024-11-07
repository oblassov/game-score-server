package main

import (
	"fmt"
	"log"
	"os"

	"game-server/v1"
)

const dbFileName = "../game.db.json"

func main() {
	store, close, err := game.FileSystemPlayerStoreFromFile(dbFileName)

	if err != nil {
		log.Fatal(err)
	}
	defer close()

	fmt.Println("Let's play poker")
	fmt.Println("Type {Name} wins to record a win")

	game.NewCLI(store, os.Stdin, game.BlindAlerterFunc(game.StdOutAlerter)).PlayGame()
}
