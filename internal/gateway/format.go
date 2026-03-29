package gateway

import (
	"regexp"
	"strings"
)

// Compiled regexes for mdToWhatsApp compiled once at startup.
var (
	// reDirective strips OpenClaw internal directive tags that should not
	// appear in user-visible messages (e.g. [[reply_to_current]],
	// [[reply_to:<id>]], [[audio_as_voice]]).
	reDirective  = regexp.MustCompile(`\[\[\s*(?:reply_to_current|reply_to_previous|reply_to\s*:\s*[^\]\n]+|audio_as_voice)\s*\]\]\s*`)
	reBold       = regexp.MustCompile(`\*\*(.+?)\*\*`)
	reItalic     = regexp.MustCompile(`\*(.+?)\*`)
	reStrike     = regexp.MustCompile(`~~(.+?)~~`)
	reHeading    = regexp.MustCompile("(?m)^#{1,3} +(.+)$")
	reBlockquote = regexp.MustCompile("(?m)^> ?")
)

// MdToWhatsApp converts Markdown formatting to WhatsApp-compatible formatting.
func MdToWhatsApp(text string) string {
	const boldMarker = "\x01"

	// Strip internal directive tags before any formatting.
	text = reDirective.ReplaceAllString(text, "")

	result := reBold.ReplaceAllString(text, boldMarker+"$1"+boldMarker)
	result = reItalic.ReplaceAllString(result, "_$1_")
	result = strings.ReplaceAll(result, boldMarker, "*")
	result = reStrike.ReplaceAllString(result, "~$1~")
	result = reHeading.ReplaceAllString(result, "*$1*")
	result = reBlockquote.ReplaceAllString(result, "")

	return result
}

// SplitMessage splits text into chunks of at most maxLen bytes.
func SplitMessage(text string, maxLen int) []string {
	if len(text) <= maxLen {
		return []string{text}
	}

	minSplit := maxLen / 4
	var chunks []string

	for len(text) > maxLen {
		chunk := text[:maxLen]

		if i := strings.LastIndex(chunk, "\n\n"); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		if i := strings.LastIndex(chunk, "\n"); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		splitPos := -1
		for _, sep := range []string{". ", "? ", "! "} {
			if i := strings.LastIndex(chunk, sep); i >= minSplit {
				pos := i + 1
				if pos > splitPos {
					splitPos = pos
				}
			}
		}
		if splitPos >= 0 {
			chunks = append(chunks, strings.TrimSpace(text[:splitPos]))
			text = strings.TrimSpace(text[splitPos:])
			continue
		}

		if i := strings.LastIndex(chunk, " "); i >= minSplit {
			chunks = append(chunks, strings.TrimSpace(text[:i]))
			text = strings.TrimSpace(text[i:])
			continue
		}

		chunks = append(chunks, strings.TrimSpace(text[:maxLen]))
		text = strings.TrimSpace(text[maxLen:])
	}

	if text != "" {
		chunks = append(chunks, strings.TrimSpace(text))
	}

	return chunks
}
