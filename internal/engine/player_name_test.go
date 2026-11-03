package engine_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/oblassov/game-score-server/internal/engine"
)

func TestValidatePlayerName(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  error
	}{
		{name: "plain name", input: "Alice", want: nil},
		{name: "exactly 64 bytes", input: strings.Repeat("a", 64), want: nil},
		{name: "multi-byte name", input: "Zoë", want: nil},
		{name: "empty", input: "", want: engine.ErrBlankName},
		{name: "invalid UTF-8", input: "\xff", want: engine.ErrInvalidUTF8Name},
		{name: "NUL", input: "Al\x00ice", want: engine.ErrControlCharName},
		{name: "ESC", input: "Al\x1bice", want: engine.ErrControlCharName},
		{name: "tab", input: "Al\tice", want: engine.ErrControlCharName},
		{name: "trailing slash", input: "Alice/", want: engine.ErrSlashName},
		{name: "inner slash", input: "a/b", want: engine.ErrSlashName},
		{name: "65 bytes", input: strings.Repeat("a", 65), want: engine.ErrNameTooLong},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := engine.ValidatePlayerName(tc.input)

			if tc.want == nil {
				if got != nil {
					t.Errorf("ValidatePlayerName(%q) = %v, want nil", tc.input, got)
				}

				return
			}

			if !errors.Is(got, tc.want) {
				t.Errorf("ValidatePlayerName(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
