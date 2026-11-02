package filesystem

import (
	"os"
	"path/filepath"
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

	newFileContents, err := os.ReadFile(file.Name())
	tests.AssertNoError(t, err)

	got := string(newFileContents)
	want := "abc"

	if got != want {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestTapeWriteReplacesFileAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.json")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatalf("could not seed file: %v", err)
	}

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	tests.AssertNoError(t, err)
	defer func() { _ = file.Close() }()

	before, err := file.Stat()
	tests.AssertNoError(t, err)

	tp := &tape{file: file}
	for _, payload := range []string{"abc", "de"} {
		if _, err = tp.Write([]byte(payload)); err != nil {
			t.Fatalf("couldn't write %q: %v", payload, err)
		}

		got, readErr := os.ReadFile(path)
		tests.AssertNoError(t, readErr)
		if string(got) != payload {
			t.Errorf("got %q want %q", got, payload)
		}
	}

	after, err := os.Stat(path)
	tests.AssertNoError(t, err)
	if os.SameFile(before, after) {
		t.Error("db file was rewritten in place, want a temp file renamed over it")
	}
	if after.Mode().Perm() != 0o644 {
		t.Errorf("file mode %v, want 0644", after.Mode().Perm())
	}

	entries, err := os.ReadDir(dir)
	tests.AssertNoError(t, err)
	if len(entries) != 1 {
		t.Errorf("found %d entries in db dir, want only the db file (leftover temp file?)", len(entries))
	}
}

func TestTapeWriteFailureLeavesFileIntact(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("read-only directories do not stop root")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "db.json")
	if err := os.WriteFile(path, []byte("12345"), 0o644); err != nil {
		t.Fatalf("could not seed file: %v", err)
	}

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	tests.AssertNoError(t, err)
	defer func() { _ = file.Close() }()

	if err = os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("could not make dir read-only: %v", err)
	}
	defer func() { _ = os.Chmod(dir, 0o700) }()

	if _, err = (&tape{file: file}).Write([]byte("abc")); err == nil {
		t.Fatal("expected an error when the temp file cannot be created")
	}

	got, err := os.ReadFile(path)
	tests.AssertNoError(t, err)
	if string(got) != "12345" {
		t.Errorf("failed write corrupted the db: got %q want %q", got, "12345")
	}
}
