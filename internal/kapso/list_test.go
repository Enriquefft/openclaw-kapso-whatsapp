package kapso

import (
	"encoding/json"
	"testing"
)

func TestInboundMessageJSON_Image(t *testing.T) {
	raw := `{
		"id": "wamid.abc",
		"type": "image",
		"from": "1234567890",
		"timestamp": "1740704195",
		"image": {
			"id": "media-img-1",
			"mime_type": "image/jpeg",
			"caption": "Look at this"
		}
	}`
	var msg InboundMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Type != "image" {
		t.Fatalf("type = %q, want image", msg.Type)
	}
	if msg.Image == nil {
		t.Fatal("Image field is nil")
	}
	if msg.Image.Caption != "Look at this" {
		t.Errorf("caption = %q, want %q", msg.Image.Caption, "Look at this")
	}
}

func TestInboundMessageJSON_Document(t *testing.T) {
	raw := `{
		"id": "wamid.def",
		"type": "document",
		"from": "1234567890",
		"timestamp": "1740704195",
		"document": {
			"id": "media-doc-1",
			"mime_type": "application/pdf",
			"filename": "invoice.pdf"
		}
	}`
	var msg InboundMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.Document == nil || msg.Document.Filename != "invoice.pdf" {
		t.Fatal("document not parsed correctly")
	}
}
