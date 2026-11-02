package inmemory

import "testing"

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
