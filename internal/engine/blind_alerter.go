package engine

import (
	"fmt"
	"io"
	"log"
	"time"
)

type BlindAlerter interface {
	ScheduleAlertAt(duration time.Duration, amount int, to io.Writer) (cancel func())
}

type BlindAlerterFunc func(duration time.Duration, amount int, to io.Writer) (cancel func())

func (a BlindAlerterFunc) ScheduleAlertAt(duration time.Duration, amount int, to io.Writer) (cancel func()) {
	return a(duration, amount, to)
}

func Alerter(duration time.Duration, amount int, to io.Writer) (cancel func()) {
	timer := time.AfterFunc(duration, func() {
		if _, err := fmt.Fprintf(to, "Blind is now %d\n", amount); err != nil {
			log.Println("couldn't print current blind: ", err)
		}
	})
	return func() { timer.Stop() }
}
