package engine

import (
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

type BlindAlerter interface {
	ScheduleAlertAt(duration time.Duration, amount int, to io.Writer) (cancel func())
}

type BlindAlerterFunc func(duration time.Duration, amount int, to io.Writer) (cancel func())

func (a BlindAlerterFunc) ScheduleAlertAt(duration time.Duration, amount int, to io.Writer) (cancel func()) {
	return a(duration, amount, to)
}

// Alerter writes the blind amount to the writer after the duration.
//
// The returned cancel func is a barrier: once it returns, the write has either
// never started (and never will) or has fully finished. If a write is in
// flight, cancel blocks until it returns. cancel is idempotent and does not
// block once the alert has finished.
//
// cancel must not be called from inside the writer's Write, as it would wait
// for itself.
func Alerter(duration time.Duration, amount int, to io.Writer) (cancel func()) {
	done := make(chan struct{})
	timer := time.AfterFunc(duration, func() {
		defer close(done)
		if _, err := fmt.Fprintf(to, "Blind is now %d\n", amount); err != nil {
			log.Println("couldn't print current blind: ", err)
		}
	})

	var once sync.Once
	return func() {
		once.Do(func() {
			if timer.Stop() {
				return
			}
			<-done
		})
	}
}
