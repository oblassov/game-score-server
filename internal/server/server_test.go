package server_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/internal/server"
	"github.com/oblassov/game-score-server/tests"
)

func TestGETPlayers(t *testing.T) {
	store := tests.StubPlayerStore{
		Scores: map[string]int{
			"Pepper": 20,
			"Floyd":  10,
		},
	}

	playerServer := mustMakePlayerServer(t, &store, tests.DummyGame)

	t.Run("return 404 on missing players", func(t *testing.T) {
		request := newGetScoreRequest("Apollo")
		response := httptest.NewRecorder()

		playerServer.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusNotFound)
		tests.AssertResponseBody(t, response.Body.String(), "0")
	})

	t.Run("returns Pepper's score", func(t *testing.T) {
		request := newGetScoreRequest("Pepper")
		response := httptest.NewRecorder()

		playerServer.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusOK)
		tests.AssertResponseBody(t, response.Body.String(), "20")
	})

	t.Run("returns Floyd's score", func(t *testing.T) {
		request := newGetScoreRequest("Floyd")
		response := httptest.NewRecorder()

		playerServer.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusOK)
		tests.AssertResponseBody(t, response.Body.String(), "10")
	})
}

func TestLeague(t *testing.T) {

	t.Run("it returns the league table as JSON", func(t *testing.T) {
		wantedLeague := engine.League{
			{Name: "Cleo", Wins: 32},
			{Name: "Chris", Wins: 20},
			{Name: "Tiest", Wins: 14},
		}

		store := tests.StubPlayerStore{League: wantedLeague}
		playerServer := mustMakePlayerServer(t, &store, tests.DummyGame)

		request := newLeagueRequest()
		response := httptest.NewRecorder()

		playerServer.ServeHTTP(response, request)

		got := getLeagueFromResponse(t, response.Body)

		tests.AssertContentType(t, response, "application/json")
		tests.AssertStatus(t, response, http.StatusOK)
		tests.AssertLeague(t, got, wantedLeague)
	})

}

func TestStoreWins(t *testing.T) {
	store := tests.StubPlayerStore{
		Scores: map[string]int{},
	}

	playerServer := mustMakePlayerServer(t, &store, tests.DummyGame)

	t.Run("it returns accepted on POST", func(t *testing.T) {
		player := "Pepper"

		request := newPostWinRequest(player)
		response := httptest.NewRecorder()

		playerServer.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusAccepted)

		tests.AssertPlayerWin(t, &store, player)
	})
}

func TestStoreWinsRejectsBadNames(t *testing.T) {
	cases := map[string]string{
		"empty name":         "",
		"whitespace only":    "%20%20",
		"tab only":           "%09",
		"invalid UTF-8":      "%FF",
		"invalid UTF-8 mid":  "Al%FFice",
		"padded invalid one": "%20%FF%20",
	}

	for name, rawName := range cases {
		t.Run(name, func(t *testing.T) {
			store := tests.StubPlayerStore{Scores: map[string]int{}}
			playerServer := mustMakePlayerServer(t, &store, tests.DummyGame)

			response := httptest.NewRecorder()
			playerServer.ServeHTTP(response, newPostWinRequest(rawName))

			tests.AssertStatus(t, response, http.StatusBadRequest)

			if len(store.WinCalls) != 0 {
				t.Errorf("expected no win recorded, got %v", store.WinCalls)
			}
		})
	}
}

func TestStoreWinsTrimsName(t *testing.T) {
	store := tests.StubPlayerStore{Scores: map[string]int{}}
	playerServer := mustMakePlayerServer(t, &store, tests.DummyGame)

	response := httptest.NewRecorder()
	playerServer.ServeHTTP(response, newPostWinRequest("%20Alice%20"))

	tests.AssertStatus(t, response, http.StatusAccepted)
	tests.AssertPlayerWin(t, &store, "Alice")
}

func TestGame(t *testing.T) {
	t.Run("Get /game returns 200", func(t *testing.T) {
		game := &tests.GameSpy{}
		playerServer := mustMakePlayerServer(t, &tests.StubPlayerStore{}, game)

		request := newGameRequest()
		response := httptest.NewRecorder()

		playerServer.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusOK)
	})

	t.Run("start game with 3 players and finish game with Ruth as a winner", func(t *testing.T) {
		wantedBlindAlert := "Blind is 100"
		winner := "Ruth"

		game := &tests.GameSpy{BlindAlert: []byte(wantedBlindAlert)}
		testServer := httptest.NewServer(mustMakePlayerServer(t, tests.DummyPlayerStore, game))
		defer testServer.Close()

		wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/ws"
		ws := mustDialWS(t, wsURL)
		closeWrapper := func() {
			if err := ws.Close(); err != nil {
				t.Errorf("couldn't close the websocket: %v", err)
			}
		}
		defer closeWrapper()

		writeMessage(t, ws, "3")
		writeMessage(t, ws, winner)

		time.Sleep(time.Millisecond * 10)
		assertGameStartedWith(t, game, 3)
		assertFinishCalledWith(t, game, winner)
		assertWebsocketGotMsg(t, ws, wantedBlindAlert)

	})
}

func retryUntil(d time.Duration, f func() bool) bool {
	deadline := time.Now().Add(d)

	for time.Now().Before(deadline) {
		if f() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}

	return false
}

func assertFinishCalledWith(t testing.TB, game *tests.GameSpy, winner string) {
	t.Helper()

	passed := retryUntil(500*time.Millisecond, func() bool {
		return game.GetFinishedWith() == winner
	})

	if !passed {
		t.Errorf("expected Finish called with %q, but got %q", winner, game.GetFinishedWith())
	}

}

func assertWebsocketGotMsg(t *testing.T, ws *websocket.Conn, want string) {
	_, gotBlindAlert, _ := ws.ReadMessage()

	if string(gotBlindAlert) != want {
		t.Errorf("got blind alert %q, wanted %q", string(gotBlindAlert), want)
	}

}

func writeMessage(t *testing.T, conn *websocket.Conn, message string) {
	t.Helper()

	if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		t.Fatalf("could not send message over ws connection %v", err)
	}

}

func mustMakePlayerServer(t *testing.T, store engine.PlayerStore, game engine.Game) *server.PlayerServer {
	playerServer, err := server.NewPlayerServer(store, game)

	if err != nil {
		t.Fatal("problem creating player server", err)
	}

	return playerServer
}

func mustDialWS(t *testing.T, url string) *websocket.Conn {

	ws, _, err := websocket.DefaultDialer.Dial(url, nil)

	if err != nil {
		t.Fatalf("could not open a ws connection on %s %v", url, err)
	}

	return ws
}

func newGetScoreRequest(name string) *http.Request {
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/players/%s", name), http.NoBody)

	if err != nil {
		fmt.Printf("did not expect error in get score %v", err)
	}

	return request
}

func newGameRequest() *http.Request {
	request, err := http.NewRequest(http.MethodGet, "/game", http.NoBody)

	if err != nil {
		fmt.Printf("did not expect error in get game %v", err)
	}

	return request
}

func newPostWinRequest(name string) *http.Request {
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/players/%s", name), http.NoBody)

	if err != nil {
		fmt.Printf("did not expect error in post win %v", err)
	}

	return request
}

func newLeagueRequest() *http.Request {
	request, err := http.NewRequest(http.MethodGet, "/league", http.NoBody)

	if err != nil {
		fmt.Printf("did not expect error in get league %v", err)
	}

	return request
}

func getLeagueFromResponse(t testing.TB, body io.Reader) (league engine.League) {
	t.Helper()

	err := json.NewDecoder(body).Decode(&league)

	if err != nil {
		t.Fatalf("Unable to parse response from server %q into slice of Player '%v'", body, err)
	}

	return
}

func assertGameStartedWith(t testing.TB, game *tests.GameSpy, numberOfPlayers int) {
	t.Helper()

	if game.GetStartedWith() != numberOfPlayers {
		t.Errorf("wanted Start called with %d, but go %d", numberOfPlayers, game.GetStartedWith())
	}

}

func TestWebSocketRejectsNonUpgradeRequest(t *testing.T) {
	playerServer := mustMakePlayerServer(t, &tests.StubPlayerStore{}, &tests.GameSpy{})
	request := httptest.NewRequest(http.MethodGet, "/ws", http.NoBody)
	response := httptest.NewRecorder()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("handler panicked on a plain GET /ws: %v", r)
		}
	}()

	playerServer.ServeHTTP(response, request)

	tests.AssertStatus(t, response, http.StatusBadRequest)
}

func TestWebSocketAcceptsBoundaryPlayerCount(t *testing.T) {
	for _, count := range []int{1, 100} {
		t.Run(fmt.Sprintf("count %d", count), func(t *testing.T) {
			game := &tests.GameSpy{}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, strconv.Itoa(count))
			writeMessage(t, ws, "Ruth")

			assertServerClosesWS(t, ws)
			assertGameStartedWith(t, game, count)
		})
	}
}

func TestWebSocketClosedAfterGameFinishes(t *testing.T) {
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, &tests.GameSpy{}))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	defer func() { _ = ws.Close() }()

	writeMessage(t, ws, "3")
	writeMessage(t, ws, "Ruth")

	assertServerClosesWS(t, ws)
}

type outputCapturingGame struct {
	mu  sync.Mutex
	out io.Writer
}

func (g *outputCapturingGame) Start(_ int, out io.Writer) (stop func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.out = out

	return func() {}
}

func (g *outputCapturingGame) Finish(string) {}

func (g *outputCapturingGame) output() io.Writer {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.out
}

func TestLateBlindAlertAfterGameEndsIsDropped(t *testing.T) {
	game := &outputCapturingGame{}
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	defer func() { _ = ws.Close() }()

	writeMessage(t, ws, "3")
	writeMessage(t, ws, "Ruth")

	assertServerClosesWS(t, ws)

	out := game.output()
	if out == nil {
		t.Fatal("game was never started")
	}

	if _, err := fmt.Fprint(out, "Blind is now 200\n"); err != nil {
		t.Errorf("late blind alert after the game ended returned an error: %v", err)
	}
}

type startRecorderGame struct {
	started atomic.Bool
}

func (g *startRecorderGame) Start(_ int, _ io.Writer) (stop func()) {
	g.started.Store(true)

	return func() {}
}

func (g *startRecorderGame) Finish(_ string) {}

func TestWebSocketRejectsInvalidPlayerCount(t *testing.T) {
	for _, count := range []string{
		"abc", "", "3 players",
		"0", "-1", "-5", "-100", "101", "1000000", "9223372036854775807",
	} {
		t.Run(fmt.Sprintf("count %q", count), func(t *testing.T) {
			game := &startRecorderGame{}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, count)

			assertServerClosesWS(t, ws)

			if game.started.Load() {
				t.Error("game was started with an invalid player count")
			}
		})
	}
}

func TestWebSocketInvalidPlayerCountSendsCloseReason(t *testing.T) {
	for _, count := range []string{"abc", "", "0", "101"} {
		t.Run(fmt.Sprintf("count %q", count), func(t *testing.T) {
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, &startRecorderGame{}))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, count)

			if err := ws.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
				t.Fatalf("could not set read deadline: %v", err)
			}

			_, _, err := ws.ReadMessage()

			var closeErr *websocket.CloseError
			if !errors.As(err, &closeErr) {
				t.Fatalf("expected a websocket close frame, got %v", err)
			}

			if closeErr.Code != websocket.CloseUnsupportedData {
				t.Errorf("close code = %d, want %d", closeErr.Code, websocket.CloseUnsupportedData)
			}

			if closeErr.Text != engine.BadPlayerInputErrMsg {
				t.Errorf("close reason = %q, want %q", closeErr.Text, engine.BadPlayerInputErrMsg)
			}
		})
	}
}

type finishSignalGame struct {
	finished chan string
}

func (g *finishSignalGame) Start(_ int, _ io.Writer) (stop func()) { return func() {} }

func (g *finishSignalGame) Finish(winner string) { g.finished <- winner }

type stopSignalGame struct {
	stopped chan struct{}
}

func (g *stopSignalGame) Start(_ int, _ io.Writer) (stop func()) {
	return func() { close(g.stopped) }
}

func (g *stopSignalGame) Finish(string) {}

func TestWebSocketStopsGameAlertsWhenHandlerReturns(t *testing.T) {
	cases := map[string]func(t *testing.T, ws *websocket.Conn){
		"client disconnects before naming a winner": func(t *testing.T, ws *websocket.Conn) {
			t.Helper()

			if err := ws.Close(); err != nil {
				t.Fatalf("could not close the client websocket: %v", err)
			}
		},
		"client sends a blank winner": func(t *testing.T, ws *websocket.Conn) {
			t.Helper()

			writeMessage(t, ws, "  ")
		},
		"game finishes normally": func(t *testing.T, ws *websocket.Conn) {
			t.Helper()

			writeMessage(t, ws, "Ruth")
		},
	}

	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			game := &stopSignalGame{stopped: make(chan struct{})}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, "3")
			act(t, ws)

			select {
			case <-game.stopped:
			case <-time.After(2 * time.Second):
				t.Error("the stop func returned by Start was not called after the handler returned")
			}
		})
	}
}

// flooderGame models the Alerter barrier: Start spawns a writer that floods the
// socket, and stop waits for that writer to return (like an in-flight alert).
type flooderGame struct {
	stopReturned chan struct{}
}

func (g *flooderGame) Start(_ int, out io.Writer) (stop func()) {
	writerDone := make(chan struct{})
	payload := make([]byte, 1<<20)

	go func() {
		defer close(writerDone)

		for {
			if _, err := out.Write(payload); err != nil {
				return
			}
		}
	}()

	return func() {
		<-writerDone
		close(g.stopReturned)
	}
}

func (g *flooderGame) Finish(string) {}

func TestWebSocketStopDoesNotHangOnClientThatStoppedReading(t *testing.T) {
	game := &flooderGame{stopReturned: make(chan struct{})}
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	defer func() { _ = ws.Close() }()

	writeMessage(t, ws, "3")
	// The client never reads, so the server's writes back up and block.
	time.Sleep(200 * time.Millisecond)
	writeMessage(t, ws, "Ruth")

	select {
	case <-game.stopReturned:
	case <-time.After(5 * time.Second):
		t.Error("stop did not return: a write to a client that stopped reading blocks forever (no write deadline)")
	}
}

func TestWebSocketClientDisconnectBeforeWinnerDoesNotFinishGame(t *testing.T) {
	game := &finishSignalGame{finished: make(chan string, 1)}
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")

	writeMessage(t, ws, "3")

	if err := ws.Close(); err != nil {
		t.Fatalf("could not close the client websocket: %v", err)
	}

	select {
	case winner := <-game.finished:
		t.Errorf("Finish was called with %q after the client disconnected without naming a winner", winner)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestWebSocketBlankWinnerDoesNotFinishGame(t *testing.T) {
	for name, blank := range map[string]string{
		"empty message":       "",
		"whitespace-only one": "  \t ",
	} {
		t.Run(name, func(t *testing.T) {
			game := &finishSignalGame{finished: make(chan string, 1)}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, "3")
			writeMessage(t, ws, blank)

			select {
			case winner := <-game.finished:
				t.Errorf("Finish was called with %q for a blank winner name", winner)
			case <-time.After(500 * time.Millisecond):
			}
		})
	}
}

func TestWebSocketTrimsWinnerName(t *testing.T) {
	for name, sent := range map[string]string{
		"surrounding spaces": " Alice ",
		"tabs and newline":   "\tAlice\n",
	} {
		t.Run(name, func(t *testing.T) {
			game := &finishSignalGame{finished: make(chan string, 1)}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, "3")
			writeMessage(t, ws, sent)

			select {
			case winner := <-game.finished:
				if winner != "Alice" {
					t.Errorf("Finish was called with %q, want %q", winner, "Alice")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Finish was not called")
			}
		})
	}
}

func assertServerClosesWS(t *testing.T, ws *websocket.Conn) {
	t.Helper()

	if err := ws.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("could not set read deadline: %v", err)
	}

	for {
		_, _, err := ws.ReadMessage()
		if err == nil {
			continue
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			t.Fatal("server kept the websocket open after the game finished")
		}

		return
	}
}

func TestPlayersRejectsUnsupportedMethod(t *testing.T) {
	playerServer := mustMakePlayerServer(t, &tests.StubPlayerStore{}, &tests.GameSpy{})
	request := httptest.NewRequest(http.MethodPut, "/players/Pepper", http.NoBody)
	response := httptest.NewRecorder()

	playerServer.ServeHTTP(response, request)

	tests.AssertStatus(t, response, http.StatusMethodNotAllowed)
}

func TestPlayersMethodNotAllowedSetsAllowHeader(t *testing.T) {
	playerServer := mustMakePlayerServer(t, &tests.StubPlayerStore{}, &tests.GameSpy{})
	request := httptest.NewRequest(http.MethodDelete, "/players/Pepper", http.NoBody)
	response := httptest.NewRecorder()

	playerServer.ServeHTTP(response, request)

	result := response.Result()
	defer func() { _ = result.Body.Close() }()

	want := "GET, POST"
	if got := result.Header.Get("Allow"); got != want {
		t.Errorf("got Allow header %q, want %q", got, want)
	}
}
