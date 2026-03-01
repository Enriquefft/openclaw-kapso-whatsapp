package transcribe

import (
	"testing"
)

func TestNormalizeMIME(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "ogg with opus codec param",
			input: "audio/ogg; codecs=opus",
			want:  "audio/ogg",
		},
		{
			name:  "audio/opus maps to audio/ogg",
			input: "audio/opus",
			want:  "audio/ogg",
		},
		{
			name:  "audio/ogg passthrough",
			input: "audio/ogg",
			want:  "audio/ogg",
		},
		{
			name:  "audio/mpeg passthrough",
			input: "audio/mpeg",
			want:  "audio/mpeg",
		},
		{
			name:  "audio/webm with params strips params",
			input: "audio/webm; codecs=vp9",
			want:  "audio/webm",
		},
		{
			name:  "empty string returns empty string",
			input: "",
			want:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeMIME(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeMIME(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestMimeToFilename(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ogg", "audio/ogg", "audio.ogg"},
		{"mpeg", "audio/mpeg", "audio.mp3"},
		{"mp4", "audio/mp4", "audio.mp4"},
		{"wav", "audio/wav", "audio.wav"},
		{"x-wav", "audio/x-wav", "audio.wav"},
		{"webm", "audio/webm", "audio.webm"},
		{"flac", "audio/flac", "audio.flac"},
		{"unknown fallback", "audio/unknown", "audio.bin"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mimeToFilename(tc.input)
			if got != tc.want {
				t.Errorf("mimeToFilename(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
