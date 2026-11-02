# Game Score Server

A Texas Hold'em poker tournament game manager and player score tracking service written in Go. The application supports both a real-time Web Server (with HTTP REST endpoints, WebSockets, and an interactive browser UI) and an interactive CLI tool.

---

## Features

- **Texas Hold'em Game Engine**: Schedules progressive blind alerts (`Blind is now <amount>`) with intervals dynamically calculated based on player count: `(5 + numberOfPlayers) * time.Minute`.
- **Real-Time WebSockets**: Live blind alerts streamed to the browser UI via WebSocket connections.
- **RESTful Player & League API**: Query individual player scores, register wins, and retrieve the current league standings sorted by wins.
- **Interactive Browser UI**: Embedded single-page web interface (`/game`) for starting games, viewing blind alerts in real time, and declaring winners.
- **Interactive CLI Tool**: Terminal-based interface for running poker sessions locally and recording wins directly to the store.
- **Thread-Safe Storage**:
  - **Filesystem Store**: Persistent JSON storage (`game.db.json`) protected by `sync.RWMutex` with atomic rewrite mechanics.
  - **In-Memory Store**: Fast, thread-safe in-memory player store for transient use and testing.

---

## Project Structure

```text
├── cmd/
│   ├── cli/             # Interactive CLI application entry point
│   └── webserver/       # HTTP and WebSocket server entry point (port 5000)
├── internal/
│   ├── app/cli/         # CLI user interaction and command parsing logic
│   ├── engine/          # Core domain models and interfaces (Game, PlayerStore, League, BlindAlerter)
│   ├── game/texasholdem/# Texas Hold'em game rules and blind schedule implementation
│   ├── server/          # HTTP server, WebSocket handler, and embedded web UI (game.html)
│   └── storage/
│       ├── filesystem/  # File-based JSON database storage with atomic rewrite tape
│       └── inmemory/    # Thread-safe in-memory storage implementation
├── tests/               # Test helpers, test doubles (spies/stubs), and fixtures
├── .golangci.yml        # Strict golangci-lint quality configuration
├── go.mod               # Go module definition (Go 1.22+)
└── README.md
```

---

## Getting Started

### Prerequisites

- [Go](https://golang.org/dl/) 1.22 or higher
- [golangci-lint](https://golangci-lint.run/) (optional, for code analysis)

### Installation

Clone the repository:

```bash
git clone git@github.com:oblassov/game-score-server.git
cd game-score-server
go mod download
```

---

## Running the Applications

### 1. Web Server

Start the web server on port `5000`:

```bash
go run ./cmd/webserver
```

Once started:
- Open your browser at [http://localhost:5000/game](http://localhost:5000/game) to play a game with real-time blind alerts.
- Check league standings at [http://localhost:5000/league](http://localhost:5000/league).

### 2. CLI Tool

Start the interactive terminal game:

```bash
go run ./cmd/cli
```

Example session:
```text
Let's play poker
Type {Name} wins to record a win
Please enter the number of players: 5
Blind is now 100
...
Alice wins
```

---

## HTTP API & WebSocket Reference

### HTTP Endpoints

| Method | Endpoint | Description | Response Codes |
|---|---|---|---|
| `GET` | `/` | Service overview and route guide | `200 OK` (plain text) |
| `GET` | `/game` | Interactive HTML/JS Poker interface | `200 OK` (HTML) |
| `GET` | `/league` | League leaderboard sorted descending by wins | `200 OK` (JSON) |
| `GET` | `/players/{name}` | Current score for player `{name}` | `200 OK` or `404 Not Found` |
| `POST` | `/players/{name}` | Increment player `{name}`'s win count | `202 Accepted` |

#### League JSON Example (`GET /league`)

```json
[
  {"Name": "Ruth", "Wins": 15},
  {"Name": "Pepper", "Wins": 10},
  {"Name": "Chris", "Wins": 5}
]
```

### WebSocket Protocol (`/ws`)

1. **Client connects** to `ws://localhost:5000/ws`.
2. **Client sends** number of players (e.g. `"5"`).
3. **Server streams** blind alerts over time (e.g. `"Blind is now 100"`, `"Blind is now 200"`).
4. **Client sends** the winner's name (e.g. `"Ruth"`).
5. **Server records** the win and finalizes the game.

---

## Testing & Quality

### Run Unit and Integration Tests

```bash
go test -v ./...
```

Run tests without cache:

```bash
go test -count=1 ./...
```

### Run Linter

```bash
golangci-lint run -c .golangci.yml
```

Or include test files:

```bash
golangci-lint run -c .golangci.yml --tests=true
```
