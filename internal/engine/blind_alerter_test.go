package engine_test

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oblassov/game-score-server/internal/engine"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestAlerterCancelStopsTheAlert(t *testing.T) {
	t.Run("alert fires when not cancelled", func(t *testing.T) {
		out := &lockedBuffer{}

		engine.Alerter(10*time.Millisecond, 100, out)

		deadline := time.Now().Add(2 * time.Second)
		for out.String() == "" && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}

		if got, want := out.String(), "Blind is now 100\n"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("alert is dropped when cancelled", func(t *testing.T) {
		out := &lockedBuffer{}

		cancel := engine.Alerter(50*time.Millisecond, 100, out)
		cancel()

		time.Sleep(150 * time.Millisecond)

		if got := out.String(); got != "" {
			t.Errorf("cancelled alert still wrote %q", got)
		}
	})
}
