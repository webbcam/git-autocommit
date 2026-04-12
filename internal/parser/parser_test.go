package parser

import "testing"

func TestStripANSI(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"no codes", "no codes"},
		{"\x1b[0mhello\x1b[0m", "hello"},
		{"\x1b[31mred\x1b[0m text", "red text"},
		{"\x1b[1;32mbold green\x1b[0m", "bold green"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := StripANSI(tt.in); got != tt.want {
			t.Errorf("StripANSI(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestExtractCommitMessage(t *testing.T) {
	const start = "===COMMIT_MSG_START==="
	const end = "===COMMIT_MSG_END==="

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid single-line message",
			input: start + "\nhello world\n" + end,
			want:  "hello world",
		},
		{
			name:  "valid multiline message",
			input: start + "\nsubject\n\nbody text\n" + end,
			want:  "subject\n\nbody text",
		},
		{
			name:  "with ANSI codes",
			input: "\x1b[0m" + start + "\nhello\n" + end + "\x1b[0m",
			want:  "hello",
		},
		{
			name:  "preamble and trailer ignored",
			input: "some preamble\n" + start + "\nmy message\n" + end + "\nsome trailer",
			want:  "my message",
		},
		{
			name:    "missing start delimiter",
			input:   "hello world\n" + end,
			wantErr: true,
		},
		{
			name:    "missing end delimiter",
			input:   start + "\nhello world",
			wantErr: true,
		},
		{
			name:    "empty between delimiters",
			input:   start + "\n" + end,
			wantErr: true,
		},
		{
			name:    "whitespace only between delimiters",
			input:   start + "\n   \n\t\n" + end,
			wantErr: true,
		},
		{
			name:    "completely empty input",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractCommitMessage(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ExtractCommitMessage() err = %v, wantErr = %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
