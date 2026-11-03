package server_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
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

	// GET must normalise the name the same way POST does (it trims), or a
	// win recorded for " Pepper " is unreachable under the same URL.
	cases := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "return 404 on missing players", path: "Apollo", wantStatus: http.StatusNotFound, wantBody: "0"},
		{name: "returns Pepper's score", path: "Pepper", wantStatus: http.StatusOK, wantBody: "20"},
		{name: "returns Floyd's score", path: "Floyd", wantStatus: http.StatusOK, wantBody: "10"},
		{name: "trims spaces around the name", path: "%20Pepper%20", wantStatus: http.StatusOK, wantBody: "20"},
		{name: "trims tabs and newlines around the name", path: "%09Floyd%0A", wantStatus: http.StatusOK, wantBody: "10"},
		{name: "keeps inner spaces", path: "Pep%20per", wantStatus: http.StatusNotFound, wantBody: "0"},
		{name: "blank name is a missing player", path: "%20%20", wantStatus: http.StatusNotFound, wantBody: "0"},
		{name: "empty name is a missing player", path: "", wantStatus: http.StatusNotFound, wantBody: "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()

			playerServer.ServeHTTP(response, newGetScoreRequest(tc.path))

			tests.AssertStatus(t, response, tc.wantStatus)
			tests.AssertResponseBody(t, response.Body.String(), tc.wantBody)
		})
	}
}

// apiContract is the status and body pinned in testdata/api/<endpoint>-<case>.json.
type apiContract struct {
	Status int    `json:"status"`
	Body   string `json:"body"`
}

func readAPIContract(t *testing.T, file string) apiContract {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", "api", file))
	tests.AssertNoError(t, err)

	var contract apiContract
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatalf("couldn't parse contract %s: %v", file, err)
	}

	return contract
}

func TestGETPlayersContract(t *testing.T) {
	store := tests.StubPlayerStore{Scores: map[string]int{"Pepper": 20}}
	playerServer := mustMakePlayerServer(t, &store, tests.DummyGame)

	cases := []struct {
		name string
		path string
		file string
	}{
		{name: "known player", path: "Pepper", file: "players-get-known.json"},
		{name: "padded name", path: "%20Pepper%20", file: "players-get-padded-name.json"},
		{name: "unknown player", path: "Apollo", file: "players-get-unknown.json"},
		{name: "blank name", path: "%20", file: "players-get-blank-name.json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := readAPIContract(t, tc.file)
			response := httptest.NewRecorder()

			playerServer.ServeHTTP(response, newGetScoreRequest(tc.path))

			tests.AssertStatus(t, response, want.Status)
			tests.AssertResponseBody(t, response.Body.String(), want.Body)
		})
	}
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

func TestPOSTPlayersContract(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		file     string
		wantWins []string
	}{
		{name: "blank", path: "%20", file: "players-post-blank.json"},
		{name: "invalid utf-8", path: "%FF", file: "players-post-invalid-utf8.json"},
		{name: "NUL control character", path: "Al%00ice", file: "players-post-control.json"},
		{name: "ESC control character", path: "Al%1Bice", file: "players-post-control.json"},
		{name: "trailing slash", path: "Alice/", file: "players-post-slash.json"},
		{name: "inner slash", path: "a/b", file: "players-post-slash.json"},
		{name: "too long", path: strings.Repeat("a", 65), file: "players-post-too-long.json"},
		{
			name: "max length", path: strings.Repeat("a", 64), file: "players-post-max-length.json",
			wantWins: []string{strings.Repeat("a", 64)},
		},
		{name: "padded", path: "%20Alice%20", file: "players-post-padded.json", wantWins: []string{"Alice"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := readAPIContract(t, tc.file)
			store := tests.StubPlayerStore{Scores: map[string]int{}}
			playerServer := mustMakePlayerServer(t, &store, tests.DummyGame)
			response := httptest.NewRecorder()

			playerServer.ServeHTTP(response, newPostWinRequest(tc.path))

			tests.AssertStatus(t, response, want.Status)
			tests.AssertResponseBody(t, response.Body.String(), want.Body)

			if !reflect.DeepEqual(store.WinCalls, tc.wantWins) {
				t.Errorf("recorded wins %q, want %q", store.WinCalls, tc.wantWins)
			}
		})
	}
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestPOSTPlayersLogsInvalidNameReason(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		wantReason string
	}{
		{name: "blank", path: "%20", wantReason: engine.ErrBlankName.Error()},
		{name: "slash", path: "Alice/", wantReason: engine.ErrSlashName.Error()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logs := &lockedBuffer{}
			prevOut := log.Writer()
			log.SetOutput(logs)
			t.Cleanup(func() { log.SetOutput(prevOut) })

			playerServer := mustMakePlayerServer(t, &tests.StubPlayerStore{}, tests.DummyGame)
			playerServer.ServeHTTP(httptest.NewRecorder(), newPostWinRequest(tc.path))

			if got := logs.String(); !strings.Contains(got, tc.wantReason) {
				t.Errorf("log %q does not contain the reason %q", got, tc.wantReason)
			}
		})
	}
}

func TestGETLeagueNilFromStore(t *testing.T) {
	want := readAPIContract(t, "league-empty.json")
	playerServer := mustMakePlayerServer(t, &tests.StubPlayerStore{League: nil}, tests.DummyGame)
	response := httptest.NewRecorder()

	playerServer.ServeHTTP(response, newLeagueRequest())

	tests.AssertStatus(t, response, want.Status)
	tests.AssertResponseBody(t, response.Body.String(), want.Body)
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

func TestGamePagePlayerCountBounds(t *testing.T) {
	playerServer := mustMakePlayerServer(t, &tests.StubPlayerStore{}, &tests.GameSpy{})

	response := httptest.NewRecorder()
	playerServer.ServeHTTP(response, newGameRequest())

	tests.AssertStatus(t, response, http.StatusOK)

	body := response.Body.String()
	inputTag := regexp.MustCompile(`<input[^>]*id="player-count"[^>]*>`).FindString(body)
	if inputTag == "" {
		t.Fatalf("no <input> tag with id=%q found in body %q", "player-count", body)
	}

	cases := []struct {
		name string
		want string
	}{
		{name: "min attribute", want: fmt.Sprintf(`min="%d"`, engine.MinPlayers)},
		{name: "max attribute", want: fmt.Sprintf(`max="%d"`, engine.MaxPlayers)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(inputTag, tc.want) {
				t.Errorf("player-count input %q does not contain %s", inputTag, tc.want)
			}
		})
	}
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
	for _, count := range []int{2, 22} {
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

// readUntilError reads messages until ReadMessage fails and returns that
// error, failing (never hanging) when the socket stays open for a second.
func readUntilError(t *testing.T, ws *websocket.Conn) error {
	t.Helper()

	if err := ws.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("could not set read deadline: %v", err)
	}

	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				t.Fatal("server kept the websocket open")
			}

			return err
		}
	}
}

func TestWebSocketCloseCodeAfterGame(t *testing.T) {
	t.Run("dropped client that stops ponging", func(t *testing.T) {
		ws := dialKeepaliveWS(t, newKeepaliveServer(t, &tests.GameSpy{}))

		ws.SetPingHandler(func(string) error { return nil })

		writeMessage(t, ws, "3")

		err := readUntilError(t, ws)

		var closeErr *websocket.CloseError
		if errors.As(err, &closeErr) && closeErr.Code == websocket.CloseNormalClosure {
			t.Errorf("got a normal closure %v after dropping a silent client, want an abnormal end", err)
		}
	})

	t.Run("saved win", func(t *testing.T) {
		testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, &tests.GameSpy{}))
		defer testServer.Close()

		ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
		defer func() { _ = ws.Close() }()

		writeMessage(t, ws, "3")
		writeMessage(t, ws, "Ruth")

		err := readUntilError(t, ws)

		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) {
			t.Fatalf("got error %v, want a *websocket.CloseError", err)
		}

		if closeErr.Code != websocket.CloseNormalClosure {
			t.Errorf("got close code %d, want %d", closeErr.Code, websocket.CloseNormalClosure)
		}
	})

	t.Run("failed save", func(t *testing.T) {
		game := &tests.GameSpy{FinishError: errors.New("disk full")}
		testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
		defer testServer.Close()

		ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
		defer func() { _ = ws.Close() }()

		writeMessage(t, ws, "3")
		writeMessage(t, ws, "Ruth")

		msg := readTextMessage(t, ws)
		for strings.HasPrefix(msg, "Blind is now ") {
			msg = readTextMessage(t, ws)
		}

		if msg != engine.RecordWinFailedMsg {
			t.Errorf("got message %q, want %q", msg, engine.RecordWinFailedMsg)
		}

		if err := ws.SetReadDeadline(time.Now().Add(200 * time.Millisecond)); err != nil {
			t.Fatalf("could not set read deadline: %v", err)
		}

		if _, _, err := ws.ReadMessage(); err != nil {
			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Timeout() {
				t.Errorf("socket closed after a failed save: %v, want it to stay open", err)
			}
		}
	})
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

func (g *outputCapturingGame) Finish(string) error { return nil }

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

var invalidPlayerCounts = []string{
	"abc", "", "3 players",
	"0", "1", "-1", "-5", "-100", "23", "100", "101", "1000000", "9223372036854775807",
}

// readTextMessage reads the next message, failing (never hanging) when none
// arrives in time or the server closed the socket instead.
func readTextMessage(t *testing.T, ws *websocket.Conn) string {
	t.Helper()

	if err := ws.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatalf("could not set read deadline: %v", err)
	}

	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("expected a text message, got %v", err)
	}

	return string(msg)
}

func assertWSMessage(t *testing.T, ws *websocket.Conn, want string) {
	t.Helper()

	if got := readTextMessage(t, ws); got != want {
		t.Errorf("got message %q, want %q", got, want)
	}
}

func assertFinishCalls(t *testing.T, game *tests.GameSpy, want ...string) {
	t.Helper()

	retryUntil(500*time.Millisecond, func() bool { return len(game.GetFinishCalls()) >= len(want) })

	got := game.GetFinishCalls()
	if len(got) != len(want) {
		t.Fatalf("Finish calls = %q, want %q", got, want)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Finish calls = %q, want %q", got, want)
			return
		}
	}
}

// Rewritten for ADR 0002: a bad count used to close the socket (1003); now the
// client gets the reason as a text message and is asked again.
func TestWebSocketRejectsInvalidPlayerCount(t *testing.T) {
	for _, count := range invalidPlayerCounts {
		t.Run(fmt.Sprintf("count %q", count), func(t *testing.T) {
			game := &tests.GameSpy{}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, count)

			assertWSMessage(t, ws, engine.BadPlayerInputErrMsg)

			if game.GetStartCalled() {
				t.Error("game was started with an invalid player count")
			}
		})
	}
}

// Rewritten for ADR 0002 (was ...SendsCloseReason): after the reason message
// the connection stays open and a valid count then a winner play the game.
func TestWebSocketInvalidPlayerCountKeepsConnectionOpen(t *testing.T) {
	for _, count := range invalidPlayerCounts {
		t.Run(fmt.Sprintf("count %q", count), func(t *testing.T) {
			game := &tests.GameSpy{}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, count)
			assertWSMessage(t, ws, engine.BadPlayerInputErrMsg)

			writeMessage(t, ws, "3")
			writeMessage(t, ws, "Ruth")

			assertFinishCalls(t, game, "Ruth")
			assertGameStartedWith(t, game, 3)
		})
	}
}

func TestWebSocketBadCountsThenGoodGame(t *testing.T) {
	game := &tests.GameSpy{}
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	defer func() { _ = ws.Close() }()

	writeMessage(t, ws, "abc")
	assertWSMessage(t, ws, engine.BadPlayerInputErrMsg)

	writeMessage(t, ws, "1")
	assertWSMessage(t, ws, engine.BadPlayerInputErrMsg)

	writeMessage(t, ws, "3")
	writeMessage(t, ws, "Ruth")

	assertFinishCalls(t, game, "Ruth")
	assertGameStartedWith(t, game, 3)
}

type finishSignalGame struct {
	finished chan string
}

func (g *finishSignalGame) Start(_ int, _ io.Writer) (stop func()) { return func() {} }

func (g *finishSignalGame) Finish(winner string) error {
	g.finished <- winner

	return nil
}

type stopSignalGame struct {
	stopped chan struct{}
}

func (g *stopSignalGame) Start(_ int, _ io.Writer) (stop func()) {
	return func() { close(g.stopped) }
}

func (g *stopSignalGame) Finish(string) error { return nil }

func TestWebSocketStopsGameAlertsWhenHandlerReturns(t *testing.T) {
	cases := map[string]func(t *testing.T, ws *websocket.Conn){
		"client disconnects before naming a winner": func(t *testing.T, ws *websocket.Conn) {
			t.Helper()

			if err := ws.Close(); err != nil {
				t.Fatalf("could not close the client websocket: %v", err)
			}
		},
		// Rewritten for ADR 0002: a blank winner no longer ends the handler, so
		// the client disconnecting after being re-prompted does.
		"client disconnects after a blank winner": func(t *testing.T, ws *websocket.Conn) {
			t.Helper()

			writeMessage(t, ws, "  ")
			assertWSMessage(t, ws, engine.ErrBlankName.Error())

			if err := ws.Close(); err != nil {
				t.Fatalf("could not close the client websocket: %v", err)
			}
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

func (g *flooderGame) Finish(string) error { return nil }

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

// assertBadWinnerRePrompts sends a valid count and the bad winner, expects the
// reason as a text message with the connection still open, then plays on with
// "Ruth" and checks Finish never saw the bad name.
func assertBadWinnerRePrompts(t *testing.T, sent, wantMsg string) {
	t.Helper()

	game := &tests.GameSpy{}
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	defer func() { _ = ws.Close() }()

	writeMessage(t, ws, "3")
	writeMessage(t, ws, sent)

	assertWSMessage(t, ws, wantMsg)

	if calls := game.GetFinishCalls(); len(calls) != 0 {
		t.Errorf("Finish was called with %q for a bad winner name", calls)
	}

	writeMessage(t, ws, "Ruth")

	assertFinishCalls(t, game, "Ruth")
}

// Rewritten for ADR 0002: a blank winner used to end the handler silently.
func TestWebSocketBlankWinnerDoesNotFinishGame(t *testing.T) {
	for name, blank := range map[string]string{
		"empty message":       "",
		"whitespace-only one": "  \t ",
		"spaces only":         "  ",
	} {
		t.Run(name, func(t *testing.T) {
			assertBadWinnerRePrompts(t, blank, engine.ErrBlankName.Error())
		})
	}
}

// Rewritten for ADR 0002: an invalid UTF-8 winner used to end the handler silently.
func TestWebSocketRejectsInvalidUTF8Winner(t *testing.T) {
	for name, sent := range map[string]string{
		"only an invalid byte":          "\xff",
		"invalid byte inside the name":  "Al\xffice",
		"truncated multibyte character": "Al\xe2\x82",
	} {
		t.Run(name, func(t *testing.T) {
			assertBadWinnerRePrompts(t, sent, engine.ErrInvalidUTF8Name.Error())
		})
	}
}

func TestWebSocketRejectsSlashInWinner(t *testing.T) {
	assertBadWinnerRePrompts(t, "Al/ice", engine.ErrSlashName.Error())
}

func TestWebSocketFailedSaveAsksAgain(t *testing.T) {
	game := &tests.GameSpy{FinishError: errors.New("disk full")}
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	defer func() { _ = ws.Close() }()

	writeMessage(t, ws, "3")
	writeMessage(t, ws, "Ruth")

	assertWSMessage(t, ws, engine.RecordWinFailedMsg)

	writeMessage(t, ws, "Ruth")

	assertFinishCalls(t, game, "Ruth", "Ruth")
	assertWSMessage(t, ws, engine.RecordWinFailedMsg)
}

func TestWebSocketTrimsPlayerCount(t *testing.T) {
	for name, sent := range map[string]string{
		"trailing space":    "4 ",
		"leading space":     " 4",
		"tabs":              "\t4\t",
		"newline":           "4\n",
		"spaces both sides": "  4  ",
	} {
		t.Run(name, func(t *testing.T) {
			game := &tests.GameSpy{}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			writeMessage(t, ws, sent)
			writeMessage(t, ws, "Ruth")

			assertServerClosesWS(t, ws)
			assertGameStartedWith(t, game, 4)
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

func TestWebSocketClosesIdleClient(t *testing.T) {
	cases := map[string]func(t *testing.T, ws *websocket.Conn){
		"never sends the player count": func(*testing.T, *websocket.Conn) {},
		"never names a winner": func(t *testing.T, ws *websocket.Conn) {
			t.Helper()

			writeMessage(t, ws, "3")
		},
	}

	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			restore := server.SetWSReadLimitsForTest(100*time.Millisecond, 100*time.Millisecond, 1024)
			defer restore()

			game := &stopSignalGame{stopped: make(chan struct{})}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			act(t, ws)

			assertServerClosesWS(t, ws)
		})
	}
}

func TestWebSocketWinnerWaitOutlastsSetupTimeout(t *testing.T) {
	restore := server.SetWSReadLimitsForTest(50*time.Millisecond, 5*time.Second, 1024)
	defer restore()

	game := &finishSignalGame{finished: make(chan string, 1)}
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	defer func() { _ = ws.Close() }()

	writeMessage(t, ws, "3")
	time.Sleep(300 * time.Millisecond)
	writeMessage(t, ws, "Ruth")

	select {
	case winner := <-game.finished:
		if winner != "Ruth" {
			t.Errorf("Finish was called with %q, want %q", winner, "Ruth")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a slow game was cut off by the setup read timeout")
	}
}

func TestWebSocketRejectsOversizedMessage(t *testing.T) {
	oversized := strings.Repeat("a", 64*1024)

	cases := map[string]func(t *testing.T, ws *websocket.Conn){
		"as the player count": func(t *testing.T, ws *websocket.Conn) {
			t.Helper()

			writeMessage(t, ws, oversized)
		},
		"as the winner": func(t *testing.T, ws *websocket.Conn) {
			t.Helper()

			writeMessage(t, ws, "3")
			writeMessage(t, ws, oversized+" wins")
		},
	}

	for name, act := range cases {
		t.Run(name, func(t *testing.T) {
			restore := server.SetWSReadLimitsForTest(5*time.Second, 5*time.Second, 1024)
			defer restore()

			game := &finishSignalGame{finished: make(chan string, 1)}
			testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
			defer testServer.Close()

			ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
			defer func() { _ = ws.Close() }()

			act(t, ws)

			assertServerClosesWS(t, ws)

			select {
			case winner := <-game.finished:
				t.Errorf("Finish was called with %q for an oversized message", winner)
			default:
			}
		})
	}
}

// Rewritten for ADR 0002: names over 64 bytes are now refused, so the limit
// is shrunk to engine.MaxPlayerNameBytes to keep testing a message exactly at it.
func TestWebSocketAcceptsMessageAtReadLimit(t *testing.T) {
	restore := server.SetWSReadLimitsForTest(5*time.Second, 5*time.Second, engine.MaxPlayerNameBytes)
	defer restore()

	game := &finishSignalGame{finished: make(chan string, 1)}
	testServer := httptest.NewServer(mustMakePlayerServer(t, &tests.StubPlayerStore{}, game))
	defer testServer.Close()

	ws := mustDialWS(t, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/ws")
	defer func() { _ = ws.Close() }()

	name := strings.Repeat("a", engine.MaxPlayerNameBytes)

	writeMessage(t, ws, "3")
	writeMessage(t, ws, name)

	select {
	case winner := <-game.finished:
		if winner != name {
			t.Errorf("Finish got a %d byte winner, want the %d byte one", len(winner), len(name))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a message exactly at the read limit was rejected")
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
