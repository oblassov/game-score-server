package server

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/oblassov/game-score-server/internal/engine"
)

//go:embed game.html
var gameHTML string
var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

const JSONContentType = "application/json"

type PlayerServer struct {
	store engine.PlayerStore
	http.Handler
	template *template.Template
	game     engine.Game

	// PingPeriod is how often /ws pings the client while waiting for the winner.
	PingPeriod time.Duration
	// PongWait is how long /ws waits for a pong before dropping the client.
	PongWait time.Duration
}

func NewPlayerServer(store engine.PlayerStore, game engine.Game) (*PlayerServer, error) {
	p := new(PlayerServer)

	tmpl, err := template.New("game.html").Parse(gameHTML)
	if err != nil {
		return nil, fmt.Errorf("problem loading template %s", err.Error())
	}

	p.game = game
	p.template = tmpl
	p.store = store
	p.PingPeriod = 30 * time.Second
	p.PongWait = 60 * time.Second

	router := http.NewServeMux()

	router.Handle("/league", http.HandlerFunc(p.leagueHandler))
	router.Handle("/players/", http.HandlerFunc(p.playersHandler))
	router.Handle("/game", http.HandlerFunc(p.playGame))
	router.Handle("/ws", http.HandlerFunc(p.webSocket))
	router.Handle("/", http.HandlerFunc(p.pageHandler))

	p.Handler = router

	return p, nil
}

func (p *PlayerServer) pageHandler(w http.ResponseWriter, _ *http.Request) {
	if _, err := fmt.Fprint(
		w,
		"Hello, run cli tool to record score!\n",
		"/players/$playername to check a player\n",
		"/league to check the league\n",
		"/game to check the game\n",
	); err != nil {
		log.Println("couldn't print the greeting: ", err)
	}
}

func (p *PlayerServer) leagueHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", JSONContentType)
	league := p.store.GetLeague()
	if league == nil {
		// Encode an empty league as [] rather than null.
		league = engine.League{}
	}
	if err := json.NewEncoder(w).Encode(league); err != nil {
		log.Println("couldn't encode the json: ", err)
	}
}

func (p *PlayerServer) playersHandler(w http.ResponseWriter, r *http.Request) {
	player := strings.TrimPrefix(r.URL.Path, "/players/")

	switch r.Method {
	case http.MethodPost:
		p.processWin(w, player)
	case http.MethodGet:
		p.showScore(w, player)
	default:
		w.Header().Set("Allow", "GET, POST")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func (p *PlayerServer) playGame(w http.ResponseWriter, _ *http.Request) {
	data := struct{ Min, Max int }{Min: engine.MinPlayers, Max: engine.MaxPlayers}
	if err := p.template.Execute(w, data); err != nil {
		log.Println("couldn't execute the template: ", err)
	}
}

func (p *PlayerServer) webSocket(w http.ResponseWriter, r *http.Request) {
	ws := newPlayerServerWS(w, r)
	if ws == nil {
		return
	}

	defer func() {
		if err := ws.Close(); err != nil {
			log.Println("couldn't close the websocket: ", err)
		}
	}()

	numberOfPlayers, ok := readPlayerCount(ws)
	if !ok {
		return
	}

	stop := p.game.Start(numberOfPlayers, ws)
	defer stop()

	// The winner deadline is fixed once, so pongs can extend the read deadline
	// but never past it.
	winnerDeadline := time.Now().Add(winnerReadTimeout)
	readDeadline := func() time.Time {
		deadline := time.Now().Add(p.PongWait)
		if winnerDeadline.Before(deadline) {
			return winnerDeadline
		}

		return deadline
	}

	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(readDeadline())
	})

	stopPings := ws.startPings(p.PingPeriod)
	defer stopPings()

	if p.playUntilFinished(ws, readDeadline) {
		if err := ws.CloseNormal(); err != nil {
			log.Println("couldn't send the close frame: ", err)
		}
	}
}

// readPlayerCount reads until the client sends a valid player count, telling
// it why each bad one was rejected. It reports false when the conversation
// ended (read or write failure).
func readPlayerCount(ws *playerServerWS) (int, bool) {
	for {
		msg, err := ws.WaitForMsg(setupReadTimeout)
		if err != nil {
			log.Println("couldn't read the numberOfPlayers: ", err)
			return 0, false
		}

		numberOfPlayers, err := strconv.Atoi(strings.TrimSpace(msg))
		if err == nil && numberOfPlayers >= engine.MinPlayers && numberOfPlayers <= engine.MaxPlayers {
			return numberOfPlayers, true
		}

		log.Println("bad numberOfPlayers, asking again")

		if !ws.reply(engine.BadPlayerInputErrMsg) {
			return 0, false
		}
	}
}

// playUntilFinished reads winners until one is valid and saved, telling the
// client why each attempt failed. It reports true only when the win was saved.
func (p *PlayerServer) playUntilFinished(ws *playerServerWS, readDeadline func() time.Time) bool {
	for {
		rawWinner, err := ws.waitForMsgUntil(readDeadline())
		if err != nil {
			log.Println("couldn't read the winner: ", err)
			return false
		}

		winner := engine.NormalizePlayerName(rawWinner)

		if nameErr := engine.ValidatePlayerName(winner); nameErr != nil {
			log.Println("bad winner, not finishing the game: ", nameErr)

			if !ws.reply(nameErr.Error()) {
				return false
			}

			continue
		}

		err = p.game.Finish(winner)
		if err == nil {
			return true
		}

		log.Println("couldn't finish the game: ", err)

		if !ws.reply(engine.RecordWinFailedMsg) {
			return false
		}
	}
}

func (p *PlayerServer) showScore(w http.ResponseWriter, rawPlayer string) {
	score := p.store.GetPlayerScore(engine.NormalizePlayerName(rawPlayer))

	if score == 0 {
		w.WriteHeader(http.StatusNotFound)
	}

	safescore := html.EscapeString(strconv.Itoa(score))

	if _, err := fmt.Fprint(w, safescore); err != nil {
		log.Println("couldn't print the score: ", err)
	}
}

func (p *PlayerServer) processWin(w http.ResponseWriter, rawPlayer string) {
	player := engine.NormalizePlayerName(rawPlayer)
	if err := engine.ValidatePlayerName(player); err != nil {
		log.Printf("couldn't record the win: invalid player name: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := p.store.RecordWin(player); err != nil {
		log.Printf("couldn't record the win: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}
