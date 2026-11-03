package inmemory

import (
	"sort"
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
	return i.store[engine.NormalizePlayerName(name)]
}

func (i *PlayerStore) RecordWin(name string) error {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.store[engine.NormalizePlayerName(name)]++
	return nil
}

func (i *PlayerStore) GetLeague() engine.League {
	i.lock.RLock()
	defer i.lock.RUnlock()

	// Non-nil even when empty, so the league encodes as [] rather than null.
	league := make(engine.League, 0, len(i.store))

	for name, wins := range i.store {
		league = append(league, engine.Player{Name: name, Wins: wins})
	}

	sort.Slice(league, func(a, b int) bool {
		if league[a].Wins != league[b].Wins {
			return league[a].Wins > league[b].Wins
		}
		return league[a].Name < league[b].Name
	})

	return league
}

func NewInMemoryPlayerStore() *PlayerStore {
	return &PlayerStore{store: map[string]int{}}
}
