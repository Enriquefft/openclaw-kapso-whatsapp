package delivery

import (
	"fmt"
	"log"
	"strings"

	"github.com/Enriquefft/openclaw-kapso-whatsapp/internal/kapso"
)

// ExtractText converts an inbound message of any supported type into a text
// representation suitable for forwarding to the gateway. It returns the text
// and true on success, or ("", false) if the message should be skipped.
// Unsupported types are logged and trigger a WhatsApp reply to the sender.
func ExtractText(msg kapso.Message, client *kapso.Client) (string, bool) {
	switch msg.Type {
	case "text":
		if msg.Text == nil {
			return "", false
		}
		return msg.Text.Body, true

	case "image":
		if msg.Image == nil {
			return "", false
		}
		return formatMediaMessage("image", msg.Image.Caption, msg.Image.MimeType, msg.Image.ID, client), true

	case "document":
		if msg.Document == nil {
			return "", false
		}
		label := msg.Document.Filename
		if label == "" {
			label = msg.Document.Caption
		}
		return formatMediaMessage("document", label, msg.Document.MimeType, msg.Document.ID, client), true

	case "audio":
		if msg.Audio == nil {
			return "", false
		}
		return formatMediaMessage("audio", "", msg.Audio.MimeType, msg.Audio.ID, client), true

	case "video":
		if msg.Video == nil {
			return "", false
		}
		return formatMediaMessage("video", msg.Video.Caption, msg.Video.MimeType, msg.Video.ID, client), true

	case "location":
		if msg.Location == nil {
			return "", false
		}
		return formatLocationMessage(msg.Location), true

	default:
		log.Printf("unsupported message type %q from %s (id=%s)", msg.Type, msg.From, msg.ID)
		go notifyUnsupported(msg.From, msg.Type, client)
		return "", false
	}
}

// formatMediaMessage builds a text representation for a media attachment.
// It attempts to retrieve the download URL from Kapso and includes it if
// available. The result is always a non-empty string.
func formatMediaMessage(kind, label, mimeType, mediaID string, client *kapso.Client) string {
	var parts []string
	parts = append(parts, "["+kind+"]")
	if label != "" {
		parts = append(parts, label)
	}
	if mimeType != "" {
		parts = append(parts, "("+mimeType+")")
	}

	// Best-effort media URL retrieval — non-fatal if it fails.
	if mediaID != "" && client != nil {
		if media, err := client.GetMediaURL(mediaID); err == nil && media.URL != "" {
			parts = append(parts, media.URL)
		} else if err != nil {
			log.Printf("could not retrieve media URL for %s: %v", mediaID, err)
		}
	}

	return strings.Join(parts, " ")
}

// formatLocationMessage builds a text representation for a location message.
func formatLocationMessage(loc *kapso.LocationContent) string {
	var parts []string
	parts = append(parts, "[location]")
	if loc.Name != "" {
		parts = append(parts, loc.Name)
	}
	if loc.Address != "" {
		parts = append(parts, loc.Address)
	}
	parts = append(parts, fmt.Sprintf("(%.6f, %.6f)", loc.Latitude, loc.Longitude))
	return strings.Join(parts, " ")
}

// notifyUnsupported sends a WhatsApp reply informing the user that their
// message type is not yet supported.
func notifyUnsupported(from, msgType string, client *kapso.Client) {
	to := from
	if !strings.HasPrefix(to, "+") {
		to = "+" + to
	}
	reply := fmt.Sprintf("Sorry, I can't process %s messages yet. Please send text instead.", msgType)
	if _, err := client.SendText(to, reply); err != nil {
		log.Printf("failed to send unsupported-type notice to %s: %v", to, err)
	}
}
