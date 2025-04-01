package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oblassov/game-score-server/internal/engine"
	"github.com/oblassov/game-score-server/internal/server"
	"github.com/oblassov/game-score-server/tests"
)

var (
	dummyGame         = &tests.GameSpy{}
	dummyBlindAlerter = &tests.SpyBlindAlerter{}
	dummyPlayerStore  = &tests.StubPlayerStore{}
	dummyStdIn        = &bytes.Buffer{}
	dummyStdOut       = &bytes.Buffer{}
)

func TestGETPlayers(t *testing.T) {
	store := tests.StubPlayerStore{
		Scores: map[string]int{
			"Pepper": 20,
			"Floyd":  10,
		},
	}

	server := mustMakePlayerServer(t, &store, dummyGame)

	t.Run("return 404 on missing players", func(t *testing.T) {
		request := newGetScoreRequest("Apollo")
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusNotFound)
	})

	t.Run("returns Pepper's score", func(t *testing.T) {
		request := newGetScoreRequest("Pepper")
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusOK)
		tests.AssertResponseBody(t, response.Body.String(), "20")
	})

	t.Run("returns Floyd's score", func(t *testing.T) {
		request := newGetScoreRequest("Floyd")
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusOK)
		tests.AssertResponseBody(t, response.Body.String(), "10")
	})
}

func TestLeague(t *testing.T) {

	t.Run("it returns the league table as JSON", func(t *testing.T) {
		wantedLeague := engine.League{
			{"Cleo", 32},
			{"Chris", 20},
			{"Tiest", 14},
		}

		store := tests.StubPlayerStore{League: wantedLeague}
		server := mustMakePlayerServer(t, &store, dummyGame)

		request := newLeagueRequest()
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

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

	server := mustMakePlayerServer(t, &store, dummyGame)

	t.Run("it returns accepted on POST", func(t *testing.T) {
		player := "Pepper"

		request := newPostWinRequest(player)
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusAccepted)

		tests.AssertPlayerWin(t, &store, player)
	})
}

func TestGame(t *testing.T) {
	t.Run("Get /game returns 200", func(t *testing.T) {
		game := &tests.GameSpy{}
		server := mustMakePlayerServer(t, &tests.StubPlayerStore{}, game)

		request := newGameRequest()
		response := httptest.NewRecorder()

		server.ServeHTTP(response, request)

		tests.AssertStatus(t, response, http.StatusOK)
	})

	t.Run("start game with 3 players and finish game with Ruth as a winner", func(t *testing.T) {
		wantedBlindAlert := "Blind is 100"
		winner := "Ruth"

		game := &tests.GameSpy{BlindAlert: []byte(wantedBlindAlert)}
		server := httptest.NewServer(mustMakePlayerServer(t, dummyPlayerStore, game))
		defer server.Close()

		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
		ws := mustDialWS(t, wsURL)
		defer ws.Close()

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
	}

	return false
}

func assertFinishCalledWith(t testing.TB, game *tests.GameSpy, winner string) {
	t.Helper()

	passed := retryUntil(500*time.Millisecond, func() bool {
		return game.FinishedWith == winner
	})

	if !passed {
		t.Errorf("expected Finish called with %q, but got %q", winner, game.FinishedWith)
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
	server, err := server.NewPlayerServer(store, game)

	if err != nil {
		t.Fatal("problem creating player server", err)
	}

	return server
}

func mustDialWS(t *testing.T, url string) *websocket.Conn {

	ws, _, err := websocket.DefaultDialer.Dial(url, nil)

	if err != nil {
		t.Fatalf("could not open a ws connection on %s %v", url, err)
	}

	return ws
}

func newGetScoreRequest(name string) *http.Request {
	request, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/players/%s", name), nil)

	if err != nil {
		fmt.Printf("did not expect error in get score %v", err)
	}

	return request
}

func newGameRequest() *http.Request {
	request, err := http.NewRequest(http.MethodGet, "/game", nil)

	if err != nil {
		fmt.Printf("did not expect error in get game %v", err)
	}

	return request
}

func newPostWinRequest(name string) *http.Request {
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("/players/%s", name), nil)

	if err != nil {
		fmt.Printf("did not expect error in post win %v", err)
	}

	return request
}

func newLeagueRequest() *http.Request {
	request, err := http.NewRequest(http.MethodGet, "/league", nil)

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

	if game.StartedWith != numberOfPlayers {
		t.Errorf("wanted Start called with %d, but go %d", numberOfPlayers, game.StartedWith)
	}

}
