package engine_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/oblassov/game-score-server/internal/engine"
)

func TestNewLeague(t *testing.T) {
	t.Run("decodes players from JSON", func(t *testing.T) {
		input := `[{"Name":"Chris","Wins":33},{"Name":"Cleo","Wins":10}]`

		got, err := engine.NewLeague(strings.NewReader(input))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := engine.League{{Name: "Chris", Wins: 33}, {Name: "Cleo", Wins: 10}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("empty array gives an empty league", func(t *testing.T) {
		got, err := engine.NewLeague(strings.NewReader(`[]`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %v, want an empty league", got)
		}
	})

	t.Run("malformed JSON returns a wrapped error", func(t *testing.T) {
		_, err := engine.NewLeague(strings.NewReader(`not json`))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}

		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Errorf("error %v does not wrap a *json.SyntaxError", err)
		}
		if !strings.Contains(err.Error(), "problem parsing a league") {
			t.Errorf("error %q lacks the context prefix", err)
		}
	})

	for name, input := range map[string]string{
		"empty input":           ``,
		"newline only":          "\n",
		"spaces only":           "  ",
		"mixed whitespace only": " \t\r\n ",
	} {
		t.Run(name+" gives an empty league", func(t *testing.T) {
			got, err := engine.NewLeague(strings.NewReader(input))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("got %v, want an empty league", got)
			}
		})
	}

	t.Run("trailing whitespace after valid JSON is fine", func(t *testing.T) {
		got, err := engine.NewLeague(strings.NewReader("[{\"Name\":\"Chris\",\"Wins\":1}]\n \n"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := engine.League{{Name: "Chris", Wins: 1}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	for name, input := range map[string]string{
		"garbage":      `[] x`,
		"second value": `[{"Name":"Chris","Wins":1}][{"Name":"Cleo","Wins":2}]`,
		"second array": "[]\n[]",
	} {
		t.Run("trailing data ("+name+") returns an error", func(t *testing.T) {
			got, err := engine.NewLeague(strings.NewReader(input))
			if err == nil {
				t.Fatalf("expected an error, got league %v", got)
			}
			if !strings.Contains(err.Error(), "problem parsing a league") {
				t.Errorf("error %q lacks the context prefix", err)
			}
		})
	}
}

func TestLeagueFind(t *testing.T) {
	league := engine.League{{Name: "Chris", Wins: 33}, {Name: "Cleo", Wins: 10}}

	t.Run("returns the matching player", func(t *testing.T) {
		got := league.Find("Cleo")
		if got == nil {
			t.Fatal("expected a player, got nil")
		}
		if got.Name != "Cleo" || got.Wins != 10 {
			t.Errorf("got %+v, want Cleo with 10 wins", *got)
		}
	})

	t.Run("returned pointer aliases the league entry", func(t *testing.T) {
		local := engine.League{{Name: "Chris", Wins: 33}}

		got := local.Find("Chris")
		if got == nil {
			t.Fatal("expected a player, got nil")
		}
		got.Wins++
		if local[0].Wins != 34 {
			t.Errorf("league entry has %d wins, want 34", local[0].Wins)
		}
	})

	t.Run("returns nil for an unknown player", func(t *testing.T) {
		if got := league.Find("Nobody"); got != nil {
			t.Errorf("got %+v, want nil", *got)
		}
	})

	t.Run("returns nil on an empty league", func(t *testing.T) {
		if got := (engine.League{}).Find("Chris"); got != nil {
			t.Errorf("got %+v, want nil", *got)
		}
	})
}
