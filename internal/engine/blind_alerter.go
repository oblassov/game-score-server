package engine

import (
	"fmt"
	"io"
	"log"
	"time"
)

type BlindAlerter interface {
	ScheduleAlertAt(duration time.Duration, amount int, to io.Writer)
}

type BlindAlerterFunc func(duration time.Duration, amount int, to io.Writer)

func (a BlindAlerterFunc) ScheduleAlertAt(duration time.Duration, amount int, to io.Writer) {
	a(duration, amount, to)
}

func Alerter(duration time.Duration, amount int, to io.Writer) {
	time.AfterFunc(duration, func() {
		if _, err := fmt.Fprintf(to, "Blind is now %d\n", amount); err != nil {
			log.Println("couldn't print current blind: ", err)
		}
	})
}
