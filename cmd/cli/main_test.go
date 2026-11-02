package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/oblassov/game-score-server/internal/app/cli"
)

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func TestRunRecordsWinInTheDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), dbFileName)
	var out strings.Builder

	if err := run(dbPath, strings.NewReader("2\nChris wins\n"), &out); err != nil {
		t.Fatalf("run returned an error: %v", err)
	}

	got, err := os.ReadFile(dbPath)
	if err != nil {
		t.Fatalf("couldn't read the database: %v", err)
	}
	if !strings.Contains(string(got), "Chris") {
		t.Errorf("database %q does not contain the winner Chris", got)
	}
	for _, want := range []string{"Let's play poker", "Type {Name} wins to record a win", cli.PlayerPrompt} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output %q does not contain %q", out.String(), want)
		}
	}
}

func TestRunRejectsBadPlayerInput(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), dbFileName)
	var out strings.Builder

	if err := run(dbPath, strings.NewReader("abc\n"), &out); err != nil {
		t.Fatalf("run returned an error: %v", err)
	}

	if !strings.Contains(out.String(), cli.BadPlayerInputErrMsg) {
		t.Errorf("output %q does not contain %q", out.String(), cli.BadPlayerInputErrMsg)
	}
}

func TestRunReturnsErrorWhenDatabaseCannotBeOpened(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing-dir", dbFileName)

	err := run(dbPath, strings.NewReader("2\nChris wins\n"), &strings.Builder{})

	if err == nil {
		t.Fatal("expected an error for an unopenable database path, got nil")
	}
}

func TestRunReturnsErrorWhenDatabaseIsCorrupt(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), dbFileName)
	if err := os.WriteFile(dbPath, []byte("not json"), 0o600); err != nil {
		t.Fatalf("couldn't seed the database: %v", err)
	}

	err := run(dbPath, strings.NewReader("2\nChris wins\n"), &strings.Builder{})

	if err == nil {
		t.Fatal("expected an error for a corrupt database, got nil")
	}
}

func TestRunReturnsInputReadError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), dbFileName)
	readErr := errors.New("stdin broke")

	err := run(dbPath, failingReader{err: readErr}, &strings.Builder{})

	if !errors.Is(err, readErr) {
		t.Errorf("got error %v, want it to wrap %v", err, readErr)
	}
}
