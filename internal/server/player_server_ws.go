package server

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
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

	return &playerServerWS{Conn: conn}
}

func (w *playerServerWS) WaitForMsg() string {
	_, msg, err := w.ReadMessage()

	if err != nil {
		log.Printf("error handling websocket %v\n", err)
	}

	return string(msg)
}

// Close marks the wrapper as closed and closes the underlying connection.
// Writes after Close are dropped. Close does not take mu, so it can still
// unblock a Write stuck on a client that stopped reading.
func (w *playerServerWS) Close() error {
	w.closed.Store(true)

	return w.Conn.Close()
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
