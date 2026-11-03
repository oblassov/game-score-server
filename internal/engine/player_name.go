package engine

import "strings"

// NormalizePlayerName is the one place player names are normalised for lookup
// and recording.
func NormalizePlayerName(raw string) string {
	return strings.TrimSpace(raw)
}
