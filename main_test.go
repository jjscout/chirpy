package main

import (
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name  string
		chirp string
		want  bool
	}{
		{
			name:  "empty chirp",
			chirp: "",
			want:  true,
		},
		{
			name:  "short chirp",
			chirp: "hello",
			want:  true,
		},
		{
			name:  "140 character chirp",
			chirp: string(make([]byte, 140)),
			want:  true,
		},
		{
			name:  "141 character chirp",
			chirp: string(make([]byte, 141)),
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validate(tt.chirp); got != tt.want {
				t.Errorf("validate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReplaceProfanities(t *testing.T) {
	blacklist := []string{"kerfuffle", "sharbert", "fornax"}

	tests := []struct {
		name      string
		message   string
		blacklist []string
		want      string
	}{
		{
			name:      "no profanities",
			message:   "hello world",
			blacklist: blacklist,
			want:      "hello world",
		},
		{
			name:      "single profanity case sensitive check",
			message:   "what a Kerfuffle this is",
			blacklist: blacklist,
			want:      "what a **** this is",
		},
		{
			name:      "multiple profanities",
			message:   "sharbert fornax",
			blacklist: blacklist,
			want:      "**** ****",
		},
		{
			name:      "profanity as substring",
			message:   "kerfuffleing is not a word",
			blacklist: blacklist,
			want:      "kerfuffleing is not a word",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replaceProfanities(tt.message, tt.blacklist); got != tt.want {
				t.Errorf("replaceProfanities() = %q, want %q", got, tt.want)
			}
		})
	}
}
