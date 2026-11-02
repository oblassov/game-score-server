package inmemory

import (
	"sync"
	"testing"
)

func TestGetLeagueSortedByWinsDescending(t *testing.T) {
	store := NewInMemoryPlayerStore()
	wins := map[string]int{"Cleo": 1, "Chris": 4, "Pepper": 2, "Ann": 3, "Bob": 5}
	for name, n := range wins {
		for range n {
			store.RecordWin(name)
		}
	}

	// Map iteration order is random, so repeat to catch an unsorted result.
	for range 50 {
		league := store.GetLeague()
		if len(league) != len(wins) {
			t.Fatalf("got %d players, want %d", len(league), len(wins))
		}
		for i := 1; i < len(league); i++ {
			if league[i-1].Wins < league[i].Wins {
				t.Fatalf("league not sorted by wins descending: %v", league)
			}
		}
	}
}

// Fails under -race if GetLeague, GetPlayerScore or RecordWin touch the map
// without the lock, and fails without -race if a win is lost.
func TestStoreConcurrentReadersAndWriters(t *testing.T) {
	const (
		writers        = 8
		winsPerWriter  = 200
		readers        = 4
		readsPerReader = 200
	)
	store := NewInMemoryPlayerStore()

	var wg sync.WaitGroup
	wg.Add(writers + readers)
	for range writers {
		go func() {
			defer wg.Done()
			for range winsPerWriter {
				store.RecordWin("Pepper")
			}
		}()
	}
	for range readers {
		go func() {
			defer wg.Done()
			for range readsPerReader {
				store.GetLeague()
				store.GetPlayerScore("Pepper")
			}
		}()
	}
	wg.Wait()

	if got, want := store.GetPlayerScore("Pepper"), writers*winsPerWriter; got != want {
		t.Errorf("got %d wins, want %d", got, want)
	}
}
