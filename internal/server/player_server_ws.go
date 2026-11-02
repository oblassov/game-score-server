package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	closeWriteTimeout = time.Second
	// messageWriteTimeout bounds a single Write so a client that stopped
	// reading cannot block the game (and the Alerter cancel barrier) forever.
	messageWriteTimeout = 2 * time.Second
	// drainTimeout and maxDrainBytes bound how long and how much the server
	// discards after an oversized message, before closing the connection.
	drainTimeout  = 500 * time.Millisecond
	maxDrainBytes = 1 << 20
)

// Read-side limits, vars so tests can shrink them.
var (
	// setupReadTimeout bounds waiting for the player-count message.
	setupReadTimeout = 30 * time.Second
	// winnerReadTimeout bounds waiting for the winner message. It must
	// outlast the longest possible game: the blind schedule has 11 blinds
	// (the last fires at 10*(5+n) minutes for n players), which is about
	// 17.5 hours for engine.MaxPlayers (100) players. 24h leaves margin.
	winnerReadTimeout = 24 * time.Hour
	// maxMessageSize is the largest inbound message, in bytes.
	maxMessageSize int64 = 1024
)

type playerServerWS struct {
	*websocket.Conn
	mu     sync.Mutex // serializes writes; never taken by Close
	closed atomic.Bool
}

func newPlayerServerWS(w http.ResponseWriter, r *http.Request) *playerServerWS {
	conn, err := wsUpgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Printf("problem upgrading connection to WebSockets %v\n", err)
		return nil
	}

	conn.SetReadLimit(maxMessageSize)

	return &playerServerWS{Conn: conn}
}

// WaitForMsg reads one message, failing if none arrives within timeout.
func (w *playerServerWS) WaitForMsg(timeout time.Duration) (string, error) {
	if err := w.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return "", fmt.Errorf("setting websocket read deadline: %w", err)
	}

	_, msg, err := w.ReadMessage()
	if err != nil {
		if errors.Is(err, websocket.ErrReadLimit) {
			w.drainInput()
		}

		return "", fmt.Errorf("reading websocket message: %w", err)
	}

	return string(msg), nil
}

// drainInput briefly discards what an oversized message's sender is still
// writing. Closing a socket with unread input resets the connection, which
// would stop the client from reading the close frame gorilla already sent.
func (w *playerServerWS) drainInput() {
	nc := w.NetConn()
	if err := nc.SetReadDeadline(time.Now().Add(drainTimeout)); err != nil {
		log.Println("couldn't set the drain deadline: ", err)
		return
	}

	_, err := io.CopyN(io.Discard, nc, maxDrainBytes)

	if err != nil && !isExpectedDrainEnd(err) {
		log.Println("couldn't drain the oversized message: ", err)
	}
}

// isExpectedDrainEnd reports whether err is how a drain normally ends: the
// client hung up or went quiet until the drain deadline.
func isExpectedDrainEnd(err error) bool {
	var netErr net.Error

	return errors.Is(err, io.EOF) || (errors.As(err, &netErr) && netErr.Timeout())
}

// Close marks the wrapper as closed and closes the underlying connection.
// Writes after Close are dropped. Close does not take mu, so it can still
// unblock a Write stuck on a client that stopped reading.
func (w *playerServerWS) Close() error {
	w.closed.Store(true)

	return w.Conn.Close()
}

// closeWithReason sends a close frame with the given code and reason so the
// client sees why the connection ended instead of an abnormal closure.
// WriteControl is safe to call concurrently with other writes.
func (w *playerServerWS) closeWithReason(code int, reason string) {
	msg := websocket.FormatCloseMessage(code, reason)
	if err := w.WriteControl(websocket.CloseMessage, msg, time.Now().Add(closeWriteTimeout)); err != nil {
		log.Println("couldn't send the websocket close frame: ", err)
	}
}

// Write sends p as a text message. Once the wrapper is closed the game is
// over, so late messages are dropped silently.
func (w *playerServerWS) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed.Load() {
		return len(p), nil
	}

	if deadlineErr := w.SetWriteDeadline(time.Now().Add(messageWriteTimeout)); deadlineErr != nil {
		return 0, fmt.Errorf("setting websocket write deadline: %w", deadlineErr)
	}

	err = w.WriteMessage(websocket.TextMessage, p)

	if err != nil {
		return 0, fmt.Errorf("writing websocket message: %w", err)
	}

	return len(p), nil
}
