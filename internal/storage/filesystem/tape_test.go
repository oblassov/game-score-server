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
	if _, err := tape.Write([]byte("abc")); err != nil {
		t.Errorf("couldn't write: %v", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Errorf("couldn't set an offset in a file: %v", err)
	}
	newFileContents, _ := io.ReadAll(file)

	got := string(newFileContents)
	want := "abc"

	if got != want {
		t.Errorf("got %v want %v", got, want)
	}
}
