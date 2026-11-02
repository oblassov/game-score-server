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
	"unicode/utf8"

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
	if err := json.NewEncoder(w).Encode(p.store.GetLeague()); err != nil {
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
	if err := p.template.Execute(w, nil); err != nil {
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

	numberOfPlayersMsg, err := ws.WaitForMsg()
	if err != nil {
		log.Println("couldn't read the numberOfPlayers: ", err)
		return
	}

	numberOfPlayers, err := strconv.Atoi(numberOfPlayersMsg)
	if err != nil {
		log.Println("couldn't convert the numberOfPlayers: ", err)
		ws.closeWithReason(websocket.CloseUnsupportedData, engine.BadPlayerInputErrMsg)
		return
	}

	if numberOfPlayers < engine.MinPlayers || numberOfPlayers > engine.MaxPlayers {
		log.Printf("numberOfPlayers %d out of range [%d, %d]", numberOfPlayers, engine.MinPlayers, engine.MaxPlayers)
		ws.closeWithReason(websocket.CloseUnsupportedData, engine.BadPlayerInputErrMsg)
		return
	}

	stop := p.game.Start(numberOfPlayers, ws)
	defer stop()

	rawWinner, err := ws.WaitForMsg()
	if err != nil {
		log.Println("couldn't read the winner: ", err)
		return
	}

	winner := strings.TrimSpace(rawWinner)
	if winner == "" {
		log.Println("blank winner, not finishing the game")
		return
	}

	p.game.Finish(winner)
}

func (p *PlayerServer) showScore(w http.ResponseWriter, player string) {
	score := p.store.GetPlayerScore(player)

	if score == 0 {
		w.WriteHeader(http.StatusNotFound)
	}

	safescore := html.EscapeString(strconv.Itoa(score))

	if _, err := fmt.Fprint(w, safescore); err != nil {
		log.Println("couldn't print the score: ", err)
	}
}

func (p *PlayerServer) processWin(w http.ResponseWriter, rawPlayer string) {
	player := strings.TrimSpace(rawPlayer)
	if player == "" || !utf8.ValidString(player) {
		log.Println("couldn't record the win: invalid player name")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	p.store.RecordWin(player)
	w.WriteHeader(http.StatusAccepted)
}
