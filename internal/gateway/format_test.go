package gateway

import (
	"testing"
)

func TestMdToWhatsApp(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bold",
			input: "this is **bold** text",
			want:  "this is *bold* text",
		},
		{
			name:  "italic standalone",
			input: "this is _italic_ text",
			want:  "this is _italic_ text",
		},
		{
			name:  "strikethrough",
			input: "this is ~~struck~~ text",
			want:  "this is ~struck~ text",
		},
		{
			name:  "heading",
			input: "## My Heading",
			want:  "*My Heading*",
		},
		{
			name:  "blockquote removed",
			input: "> quoted text",
			want:  "quoted text",
		},
		{
			name:  "strip reply_to_current",
			input: "[[reply_to_current]] Hello!",
			want:  "Hello!",
		},
		{
			name:  "strip reply_to_current with extra spaces",
			input: "[[ reply_to_current ]] Hello!",
			want:  "Hello!",
		},
		{
			name:  "strip reply_to_previous",
			input: "[[reply_to_previous]] Hello!",
			want:  "Hello!",
		},
		{
			name:  "strip reply_to with id",
			input: "[[reply_to:msg-123]] Hello!",
			want:  "Hello!",
		},
		{
			name:  "strip reply_to with id and spaces",
			input: "[[ reply_to: msg-123 ]] Hello!",
			want:  "Hello!",
		},
		{
			name:  "strip audio_as_voice",
			input: "[[audio_as_voice]] Here is the audio",
			want:  "Here is the audio",
		},
		{
			name:  "strip directive with formatting",
			input: "[[reply_to_current]] ✅ **Done.** Check it out.",
			want:  "✅ *Done.* Check it out.",
		},
		{
			name:  "no directive passthrough",
			input: "Just a normal message",
			want:  "Just a normal message",
		},
		{
			name:  "preserve non-directive brackets",
			input: "[not a directive] hello",
			want:  "[not a directive] hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MdToWhatsApp(tt.input)
			if got != tt.want {
				t.Errorf("MdToWhatsApp(%q)\n got: %q\nwant: %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   int // expected number of chunks
	}{
		{
			name:   "short message no split",
			input:  "hello",
			maxLen: 100,
			want:   1,
		},
		{
			name:   "long message splits",
			input:  "word " + string(make([]byte, 200)),
			maxLen: 100,
			want:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitMessage(tt.input, tt.maxLen)
			if len(got) != tt.want {
				t.Errorf("SplitMessage() returned %d chunks, want %d", len(got), tt.want)
			}
		})
	}
}
