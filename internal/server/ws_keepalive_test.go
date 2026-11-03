package server_test

import (
	"log"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oblassov/game-score-server/internal/server"
	"github.com/oblassov/game-score-server/tests"
)

const (
	keepalivePingPeriod = 50 * time.Millisecond
	keepalivePongWait   = 150 * time.Millisecond
)

// newKeepaliveServer starts a /ws server whose keepalive timings are short
// enough for a test to see several ping periods.
func newKeepaliveServer(t *testing.T, game *tests.GameSpy) *httptest.Server {
	t.Helper()

	playerServer := mustMakePlayerServer(t, &tests.StubPlayerStore{}, game)
	playerServer.PingPeriod = keepalivePingPeriod
	playerServer.PongWait = keepalivePongWait

	testServer := httptest.NewServer(playerServer)
	t.Cleanup(testServer.Close)

	return testServer
}

func dialKeepaliveWS(t *testing.T, testServer *httptest.Server) *websocket.Conn {
	t.Helper()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	t.Cleanup(func() { _ = ws.Close() })

	return ws
}

// readInBackground reads until the connection fails, so gorilla processes the
// control frames (pings) the server sends. The returned channel yields the
// read error once the connection is gone.
func readInBackground(ws *websocket.Conn) <-chan error {
	done := make(chan error, 1)

	go func() {
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				done <- err

				return
			}
		}
	}()

	return done
}

func ponger(ws *websocket.Conn, pings *atomic.Int32) func(string) error {
	return func(appData string) error {
		pings.Add(1)

		return ws.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
	}
}

func assertConnectionClosedWithin(t *testing.T, done <-chan error, d time.Duration, what string) {
	t.Helper()

	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("%s: the server kept the websocket open for %v", what, d)
	}
}

func assertNoFinish(t *testing.T, game *tests.GameSpy) {
	t.Helper()

	if calls := game.GetFinishCalls(); len(calls) != 0 {
		t.Errorf("Finish was called with %q, want no call", calls)
	}
}

func TestWebSocketSendsPings(t *testing.T) {
	game := &tests.GameSpy{}
	ws := dialKeepaliveWS(t, newKeepaliveServer(t, game))

	var pings atomic.Int32

	ws.SetPingHandler(ponger(ws, &pings))
	readInBackground(ws)

	writeMessage(t, ws, "3")

	if !retryUntil(time.Second, func() bool { return pings.Load() >= 2 }) {
		t.Errorf("client saw %d pings within 1s, want at least 2", pings.Load())
	}
}

func TestWebSocketDropsClientThatStopsPonging(t *testing.T) {
	game := &tests.GameSpy{}
	ws := dialKeepaliveWS(t, newKeepaliveServer(t, game))

	ws.SetPingHandler(func(string) error { return nil })
	done := readInBackground(ws)

	writeMessage(t, ws, "3")

	assertConnectionClosedWithin(t, done, time.Second, "client never answered a ping")
	assertNoFinish(t, game)
}

func TestWebSocketKeepsClientThatPongs(t *testing.T) {
	game := &tests.GameSpy{}
	ws := dialKeepaliveWS(t, newKeepaliveServer(t, game))

	var pings atomic.Int32

	ws.SetPingHandler(ponger(ws, &pings))
	done := readInBackground(ws)

	writeMessage(t, ws, "3")
	time.Sleep(4 * keepalivePongWait)

	select {
	case err := <-done:
		t.Fatalf("the server dropped a client that answers pings: %v", err)
	default:
	}

	writeMessage(t, ws, "Ruth")

	assertFinishCalledWith(t, game, "Ruth")
}

func TestWebSocketWinnerDeadlineCapsPongs(t *testing.T) {
	restore := server.SetWSReadLimitsForTest(5*time.Second, 300*time.Millisecond, 1024)
	defer restore()

	game := &tests.GameSpy{}
	ws := dialKeepaliveWS(t, newKeepaliveServer(t, game))

	var pings atomic.Int32

	ws.SetPingHandler(ponger(ws, &pings))
	done := readInBackground(ws)

	writeMessage(t, ws, "3")

	assertConnectionClosedWithin(t, done, 1500*time.Millisecond, "pongs kept arriving past the winner deadline")
	assertNoFinish(t, game)
}

func TestWebSocketOversizedDrainCapIsLogged(t *testing.T) {
	logs := &lockedBuffer{}
	prevOut := log.Writer()
	log.SetOutput(logs)
	t.Cleanup(func() { log.SetOutput(prevOut) })

	game := &tests.GameSpy{}
	ws := dialKeepaliveWS(t, newKeepaliveServer(t, game))

	// gorilla's default close handler answers the server's close frame, which
	// makes every later client write fail with ErrCloseSent. Ignore the close
	// frame so the client keeps sending while the server drains.
	ws.SetCloseHandler(func(int, string) error { return nil })

	// More than the read limit plus the 1 MiB drain cap, so the drain stops
	// at the cap with input still unread.
	oversized := []byte(strings.Repeat("a", 2<<20))

	go func() {
		_ = ws.WriteMessage(websocket.TextMessage, oversized)
	}()

	assertServerClosesWS(t, ws)

	// Other drain messages (deadline, read failure) never mention the cap.
	hitCap := func() bool {
		out := logs.String()

		return strings.Contains(out, "drain") && strings.Contains(out, "cap")
	}

	if !retryUntil(time.Second, hitCap) {
		t.Errorf("log %q does not mention the drain hitting its cap", logs.String())
	}
}
