package poker

import (
	"io"
	"os"
)

type tape struct {
	file *os.File
}

func (t *tape) Write(p []byte) (n int, err error) {
	if err := t.file.Truncate(0); err != nil {
		return 0, err
	}
	if _, err := t.file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}
	return t.file.Write(p)
}
