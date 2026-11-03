BIN := bin

.PHONY: help build run server cli test test-web lint clean

help:
	@echo "make server  - build and run the web server (http://localhost:5000)"
	@echo "make cli     - build and run the poker CLI"
	@echo "make build   - build both binaries into $(BIN)/"
	@echo "make test    - run tests with the race detector"
	@echo "make test-web - run the game page tests with node"
	@echo "make lint    - run golangci-lint (including tests)"
	@echo "make clean   - remove $(BIN)/"

build:
	go build -o $(BIN)/webserver ./cmd/webserver
	go build -o $(BIN)/cli ./cmd/cli

# The league db (game.db.json) is created in the working directory (the repo root here), or at GAME_DB_PATH.
run: server

server:
	go build -o $(BIN)/webserver ./cmd/webserver
	./$(BIN)/webserver

cli:
	go build -o $(BIN)/cli ./cmd/cli
	./$(BIN)/cli

test:
	go test -race -count=1 ./...

test-web:
	node --test internal/server/game_page.test.mjs

lint:
	golangci-lint run --tests=true ./...

clean:
	rm -rf $(BIN)
