package transcribe

import "strings"

// NormalizeMIME strips MIME type parameters and maps variants to canonical forms.
// Examples: "audio/ogg; codecs=opus" -> "audio/ogg", "audio/opus" -> "audio/ogg".
func NormalizeMIME(mimeType string) string {
	if mimeType == "" {
		return ""
	}
	// Strip parameters (everything after the first semicolon).
	norm, _, _ := strings.Cut(mimeType, ";")
	norm = strings.ToLower(strings.TrimSpace(norm))

	// Map known aliases to canonical forms.
	switch norm {
	case "audio/opus":
		return "audio/ogg"
	}
	return norm
}

// mimeToFilename returns an appropriate filename for the given normalized MIME type.
func mimeToFilename(norm string) string {
	switch norm {
	case "audio/ogg":
		return "audio.ogg"
	case "audio/mpeg":
		return "audio.mp3"
	case "audio/mp4":
		return "audio.mp4"
	case "audio/wav", "audio/x-wav":
		return "audio.wav"
	case "audio/webm":
		return "audio.webm"
	case "audio/flac":
		return "audio.flac"
	default:
		return "audio.bin"
	}
}
