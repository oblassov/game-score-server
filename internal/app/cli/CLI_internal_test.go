package cli

import "testing"

func TestExtractWinnerTable(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "valid", input: "Chris wins", want: "Chris"},
		{name: "valid multi-word name", input: "Chris Smith wins", want: "Chris Smith"},
		{name: "extra space before wins is trimmed", input: "Chris  wins", want: "Chris"},
		{name: "leading spaces are trimmed", input: "  Chris wins", want: "Chris"},
		{name: "leading tab and extra spaces are trimmed", input: "\t Chris Smith  wins", want: "Chris Smith"},
		{name: "whitespace-only name", input: "   wins", wantErr: true},
		{name: "bare wins", input: "wins", wantErr: true},
		{name: "wins with empty name", input: " wins", wantErr: true},
		{name: "wins first", input: "wins Chris", wantErr: true},
		{name: "wins as a prefix of a word", input: "Cleo winsome", wantErr: true},
		{name: "wins in the middle", input: "Cleo wins again", wantErr: true},
		{name: "no wins", input: "Cleo is the best", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractWinner(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("extractWinner(%q) = %q, want an error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("extractWinner(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("extractWinner(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractWinnerBadInputMessage(t *testing.T) {
	_, err := extractWinner("Cleo is the best")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if err.Error() != BadWinnerInputErrMsg {
		t.Errorf("got error %q, want %q", err.Error(), BadWinnerInputErrMsg)
	}
}
