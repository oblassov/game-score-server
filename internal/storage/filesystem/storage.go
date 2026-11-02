package filesystem

import (
	"encoding/json"
	"errors"
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

func PlayerStoreFromFile(path string) (*PlayerStore, func() error, error) {

	db, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o666)
	if err != nil {
		return nil, nil, fmt.Errorf("problem opening %s: %w", path, err)
	}

	closeFunc := func() error {
		if closeErr := db.Close(); closeErr != nil {
			return fmt.Errorf("couldn't close %s: %w", path, closeErr)
		}
		return nil
	}

	store, err := NewPlayerStore(db)
	if err != nil {
		return nil, nil, errors.Join(
			fmt.Errorf("problem creating file system store: %w", err),
			closeFunc(),
		)
	}

	return store, closeFunc, nil
}

func NewPlayerStore(file *os.File) (*PlayerStore, error) {

	err := initializePlayerDBFile(file)

	if err != nil {
		return nil, fmt.Errorf("problem initializing player db file: %w", err)
	}

	league, err := engine.NewLeague(file)

	if err != nil {
		return nil, fmt.Errorf("problem loading player store from file %s: %w", file.Name(), err)
	}

	return &PlayerStore{
		database: json.NewEncoder(&tape{file}),
		league:   league,
	}, nil

}

func initializePlayerDBFile(file *os.File) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("couldn't set an offset in a file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("problem getting file info from file %s, %w", file.Name(), err)
	}

	if info.Size() == 0 {
		if _, err := file.WriteString("[]"); err != nil {
			return fmt.Errorf("couldn't write string: %w", err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("couldn't set an offset in a file when size is 0: %w", err)
		}
	}

	return nil
}

func (f *PlayerStore) GetLeague() engine.League {
	f.lock.RLock()
	defer f.lock.RUnlock()

	league := make(engine.League, len(f.league))
	copy(league, f.league)
	sort.Slice(league, func(i, j int) bool {
		return league[i].Wins > league[j].Wins
	})

	return league
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

	if err := f.database.Encode(f.league); err != nil {
		log.Printf("couldn't encode the db: %v", err)

		if player != nil {
			player.Wins--
		} else {
			f.league = f.league[:len(f.league)-1]
		}
	}
}
