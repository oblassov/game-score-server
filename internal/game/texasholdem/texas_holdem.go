package texasholdem

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/oblassov/game-score-server/internal/engine"
)

type TexasHoldem struct {
	store   engine.PlayerStore
	alerter engine.BlindAlerter
}

func NewTexasHoldem(store engine.PlayerStore, alerter engine.BlindAlerter) *TexasHoldem {
	return &TexasHoldem{
		store:   store,
		alerter: alerter,
	}
}

func (p *TexasHoldem) Start(numberOfPlayers int, alertsDestination io.Writer) (stop func()) {
	blindIncrement := time.Duration(5+numberOfPlayers) * time.Minute
	blinds := []int{100, 200, 300, 400, 500, 600, 800, 1000, 2000, 4000, 8000}
	blindTime := 0 * time.Minute
	cancels := make([]func(), 0, len(blinds))

	for _, blind := range blinds {
		cancels = append(cancels, p.alerter.ScheduleAlertAt(blindTime, blind, alertsDestination))
		blindTime += blindIncrement
	}

	return func() {
		for _, cancel := range cancels {
			if cancel != nil {
				cancel()
			}
		}
	}
}

func (p *TexasHoldem) Finish(winner string) error {
	if err := p.store.RecordWin(winner); err != nil {
		log.Printf("couldn't record win for %q: %v", winner, err)

		return fmt.Errorf("record win for %q: %w", winner, err)
	}

	return nil
}
