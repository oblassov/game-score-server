package inmemory

import (
	"sync"

	"github.com/oblassov/game-score-server/internal/engine"
)

type PlayerStore struct {
	store map[string]int
	lock  sync.RWMutex
}

func (i *PlayerStore) GetPlayerScore(name string) int {
	i.lock.RLock()
	defer i.lock.RUnlock()
	return i.store[name]
}

func (i *PlayerStore) RecordWin(name string) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.store[name]++
}

func (i *PlayerStore) GetLeague() engine.League {
	var league []engine.Player

	for name, wins := range i.store {
		league = append(league, engine.Player{name, wins})
	}

	return league
}

func NewInMemoryPlayerStore() *PlayerStore {
	return &PlayerStore{store: map[string]int{}}
}
