package inmemory

import "testing"

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

// Fails only under -race: GetLeague reads the map without the lock.
func TestGetLeagueConcurrentWithRecordWin(_ *testing.T) {
	store := NewInMemoryPlayerStore()

	done := make(chan struct{})
	go func() {
		defer close(done)
		store.RecordWin("Pepper")
	}()

	store.GetLeague()
	<-done
}
