package engine

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxPlayerNameBytes is the longest accepted player name, in bytes.
const MaxPlayerNameBytes = 64

var (
	ErrBlankName       = errors.New("player name is blank")
	ErrInvalidUTF8Name = errors.New("player name is not valid UTF-8")
	ErrControlCharName = errors.New("player name contains a control character")
	ErrSlashName       = errors.New("player name contains '/'")
	ErrNameTooLong     = errors.New("player name is longer than 64 bytes")
)

// NormalizePlayerName is the one place player names are normalised for lookup
// and recording.
func NormalizePlayerName(raw string) string {
	return strings.TrimSpace(raw)
}

// ValidatePlayerName reports why an already normalised name is not acceptable.
func ValidatePlayerName(name string) error {
	switch {
	case name == "":
		return ErrBlankName
	case !utf8.ValidString(name):
		return ErrInvalidUTF8Name
	case len(name) > MaxPlayerNameBytes:
		return ErrNameTooLong
	case strings.ContainsFunc(name, unicode.IsControl):
		return ErrControlCharName
	case strings.Contains(name, "/"):
		return ErrSlashName
	}

	return nil
}
