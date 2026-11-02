package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type Player struct {
	Name string
	Wins int
}

type League []Player

func (l League) Find(name string) *Player {

	for i, p := range l {
		if p.Name == name {
			return &l[i]
		}
	}

	return nil
}

func NewLeague(reader io.Reader) (League, error) {
	var league League
	dec := json.NewDecoder(reader)

	err := dec.Decode(&league)
	if errors.Is(err, io.EOF) {
		// Empty or whitespace-only input is an empty league.
		return league, nil
	}
	if err != nil {
		return league, fmt.Errorf("problem parsing a league: %w", err)
	}

	// Anything but whitespace after the first value is an error.
	_, err = dec.Token()
	if errors.Is(err, io.EOF) {
		return league, nil
	}
	if err == nil {
		err = errors.New("unexpected data after top-level value")
	}

	return league, fmt.Errorf("problem parsing a league: %w", err)
}
