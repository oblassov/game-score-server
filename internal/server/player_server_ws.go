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
	// 4.5 hours for engine.MaxPlayers (22) players. 24h leaves margin.
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
	return w.waitForMsgUntil(time.Now().Add(timeout))
}

// waitForMsgUntil reads one message, failing if none arrives by deadline.
func (w *playerServerWS) waitForMsgUntil(deadline time.Time) (string, error) {
	if err := w.SetReadDeadline(deadline); err != nil {
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

	if err == nil {
		log.Printf("drain hit its cap of %d bytes with input still unread", maxDrainBytes)
		return
	}

	if !isExpectedDrainEnd(err) {
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

// CloseNormal sends a close frame with code 1000 so the client can tell a
// finished game from a dropped connection. Like ping, it shares Write's mutex
// and write deadline; once the wrapper is closed it does nothing.
func (w *playerServerWS) CloseNormal() error {
	msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")

	return w.writeFrame(websocket.CloseMessage, msg, "close frame")
}

// writeFrame is the one write path shared by Write, ping and CloseNormal: it
// holds mu, sets the write deadline and writes one frame. Once the wrapper is
// closed it drops the frame and returns nil. what names the frame in errors.
func (w *playerServerWS) writeFrame(messageType int, data []byte, what string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed.Load() {
		return nil
	}

	if err := w.SetWriteDeadline(time.Now().Add(messageWriteTimeout)); err != nil {
		return fmt.Errorf("setting websocket write deadline: %w", err)
	}

	if err := w.WriteMessage(messageType, data); err != nil {
		return fmt.Errorf("writing websocket %s: %w", what, err)
	}

	return nil
}

// reply sends msg as a text message and reports whether it was written.
func (w *playerServerWS) reply(msg string) bool {
	if _, err := w.Write([]byte(msg)); err != nil {
		log.Println("couldn't send the reply: ", err)
		return false
	}

	return true
}

// startPings pings the client every period until the returned stop func is
// called or a ping cannot be written. Pings share Write's mutex and write
// deadline, so they never interleave with a blind alert.
func (w *playerServerWS) startPings(period time.Duration) (stop func()) {
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(period)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := w.ping(); err != nil {
					log.Println("couldn't ping the client: ", err)
					return
				}
			}
		}
	}()

	return sync.OnceFunc(func() { close(done) })
}

// ping sends one ping frame. Once the wrapper is closed it does nothing.
func (w *playerServerWS) ping() error {
	return w.writeFrame(websocket.PingMessage, nil, "ping")
}

// Write sends p as a text message. Once the wrapper is closed the game is
// over, so late messages are dropped silently.
func (w *playerServerWS) Write(p []byte) (int, error) {
	if err := w.writeFrame(websocket.TextMessage, p, "message"); err != nil {
		return 0, err
	}

	return len(p), nil
}
