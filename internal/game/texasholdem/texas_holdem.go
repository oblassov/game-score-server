package texasholdem

import (
	"io"
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

func (p *TexasHoldem) Start(numberOfPlayers int, alertsDestination io.Writer) {
	blindIncrement := time.Duration(5+numberOfPlayers) * time.Minute
	blinds := []int{100, 200, 300, 400, 500, 600, 800, 1000, 2000, 4000, 8000}
	blindTime := 0 * time.Minute

	for _, blind := range blinds {
		p.alerter.ScheduleAlertAt(blindTime, blind, alertsDestination)
		blindTime += blindIncrement
	}
}

func (p *TexasHoldem) Finish(winner string) {
	p.store.RecordWin(winner)
}
