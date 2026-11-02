package server

import "time"

// SetWSReadLimitsForTest overrides the /ws read deadlines and the maximum
// message size, and returns a func that restores the previous values.
func SetWSReadLimitsForTest(setup, winner time.Duration, maxMessageBytes int64) (restore func()) {
	oldSetup, oldWinner, oldMax := setupReadTimeout, winnerReadTimeout, maxMessageSize

	setupReadTimeout, winnerReadTimeout, maxMessageSize = setup, winner, maxMessageBytes

	return func() {
		setupReadTimeout, winnerReadTimeout, maxMessageSize = oldSetup, oldWinner, oldMax
	}
}
