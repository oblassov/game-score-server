package engine_test

import (
	"errors"
	"io"
	"log"
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

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestAlerterLogsWriteError(t *testing.T) {
	logs := &lockedBuffer{}
	prevOut := log.Writer()
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(prevOut) })

	writeErr := errors.New("disk on fire")
	engine.Alerter(time.Millisecond, 100, failingWriter{err: writeErr})

	deadline := time.Now().Add(2 * time.Second)
	for logs.String() == "" && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}

	got := logs.String()
	for _, want := range []string{"couldn't print current blind", writeErr.Error()} {
		if !strings.Contains(got, want) {
			t.Errorf("log %q does not contain %q", got, want)
		}
	}
}

func TestBlindAlerterFuncForwardsToTheFunction(t *testing.T) {
	var (
		gotDuration time.Duration
		gotAmount   int
		gotWriter   io.Writer
		cancelled   bool
	)

	alerter := engine.BlindAlerterFunc(func(d time.Duration, amount int, to io.Writer) func() {
		gotDuration, gotAmount, gotWriter = d, amount, to

		return func() { cancelled = true }
	})

	out := &lockedBuffer{}
	cancel := alerter.ScheduleAlertAt(3*time.Second, 200, out)

	if gotDuration != 3*time.Second || gotAmount != 200 || gotWriter != io.Writer(out) {
		t.Errorf("got (%v, %d, %v), want (3s, 200, out)", gotDuration, gotAmount, gotWriter)
	}

	cancel()

	if !cancelled {
		t.Error("returned cancel func is not the one from the wrapped function")
	}
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
