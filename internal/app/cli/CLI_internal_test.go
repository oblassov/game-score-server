package cli

import "testing"

func TestExtractWinnerBadInputMessage(t *testing.T) {
	_, err := extractWinner("Cleo is the best")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if err.Error() != BadWinnerInputErrMsg {
		t.Errorf("got error %q, want %q", err.Error(), BadWinnerInputErrMsg)
	}
}
