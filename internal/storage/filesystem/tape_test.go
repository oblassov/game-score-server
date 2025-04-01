package filesystem

import (
	"io"
	"testing"

	"github.com/oblassov/game-score-server/tests"
)

func TestTape_Write(t *testing.T) {
	file, clean := tests.CreateTempFile(t, "12345")
	defer clean()

	tape := &tape{file: file}
	tape.Write([]byte("abc"))

	file.Seek(0, io.SeekStart)
	newFileContents, _ := io.ReadAll(file)

	got := string(newFileContents)
	want := "abc"

	if got != want {
		t.Errorf("got %v want %v", got, want)
	}
}
