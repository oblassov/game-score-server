package filesystem

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"sync"

	"github.com/oblassov/game-score-server/internal/engine"
)

type PlayerStore struct {
	database *json.Encoder
	league   engine.League
	lock     sync.RWMutex
}

func PlayerStoreFromFile(path string) (*PlayerStore, func(), error) {

	db, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)

	if err != nil {
		log.Fatalf("problem opening %s %v", path, err)
	}

	closeFunc := func() {
		db.Close()
	}

	store, err := NewPlayerStore(db)

	if err != nil {
		log.Fatalf("problem creating file system store, %v", err)
	}

	return store, closeFunc, err
}

func NewPlayerStore(file *os.File) (*PlayerStore, error) {

	err := initializePlayerDBFile(file)

	if err != nil {
		return nil, fmt.Errorf("problem initializing player db file %v", err)
	}

	league, err := engine.NewLeague(file)

	if err != nil {
		return nil, fmt.Errorf("problem loading player store from file %s, %v", file.Name(), err)
	}

	return &PlayerStore{
		database: json.NewEncoder(&tape{file}),
		league:   league,
	}, nil

}

func initializePlayerDBFile(file *os.File) error {
	file.Seek(0, io.SeekStart)

	info, err := file.Stat()

	if err != nil {
		return fmt.Errorf("problem getting file info from file %s, %v", file.Name(), err)
	}

	if info.Size() == 0 {
		file.Write([]byte("[]"))
		file.Seek(0, io.SeekStart)
	}

	return nil

}

func (f *PlayerStore) GetLeague() engine.League {
	f.lock.RLock()
	defer f.lock.RUnlock()
	sort.Slice(f.league, func(i, j int) bool {
		return f.league[i].Wins > f.league[j].Wins
	})
	return f.league
}

func (f *PlayerStore) GetPlayerScore(name string) int {
	f.lock.RLock()
	defer f.lock.RUnlock()

	player := f.league.Find(name)

	if player != nil {
		return player.Wins
	}

	return 0
}

func (f *PlayerStore) RecordWin(name string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	player := f.league.Find(name)

	if player != nil {
		player.Wins++
	} else {
		f.league = append(f.league, engine.Player{Name: name, Wins: 1})
	}

	f.database.Encode(f.league)
}
