package server

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const closeWriteTimeout = time.Second

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

	return &playerServerWS{Conn: conn}
}

func (w *playerServerWS) WaitForMsg() (string, error) {
	_, msg, err := w.ReadMessage()
	if err != nil {
		return "", fmt.Errorf("reading websocket message: %w", err)
	}

	return string(msg), nil
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

	err = w.WriteMessage(websocket.TextMessage, p)

	if err != nil {
		return 0, err
	}

	return len(p), nil
}
