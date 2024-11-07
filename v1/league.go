package game

import (
	"encoding/json"
	"fmt"
	"io"
)

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
	err := json.NewDecoder(reader).Decode(&league)

	if err != nil {
		err = fmt.Errorf("problem parsing a league, %v", err)
	}

	return league, err
}
