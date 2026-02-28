package kapso

// Meta-standard WhatsApp webhook types (used by Kapso).

// WebhookPayload is the top-level webhook delivery from Kapso.
type WebhookPayload struct {
Object string  `json:"object"`
Entry  []Entry `json:"entry"`
}

// Entry represents one business account entry.
type Entry struct {
ID      string   `json:"id"`
Changes []Change `json:"changes"`
}

// Change wraps a single change notification.
type Change struct {
Field string      `json:"field"`
Value ChangeValue `json:"value"`
}

// ChangeValue holds the message data.
type ChangeValue struct {
MessagingProduct string    `json:"messaging_product"`
Metadata         Metadata  `json:"metadata"`
Contacts         []Contact `json:"contacts,omitempty"`
Messages         []Message `json:"messages,omitempty"`
Statuses         []Status  `json:"statuses,omitempty"`
}

// Metadata about the receiving phone number.
type Metadata struct {
DisplayPhoneNumber string `json:"display_phone_number"`
PhoneNumberID      string `json:"phone_number_id"`
}

// Contact is a WhatsApp contact.
type Contact struct {
Profile ContactProfile `json:"profile"`
WaID    string         `json:"wa_id"`
}

// ContactProfile has the display name.
type ContactProfile struct {
Name string `json:"name"`
}

// Message represents an incoming WhatsApp message.
type Message struct {
From      string           `json:"from"`
ID        string           `json:"id"`
Timestamp string           `json:"timestamp"`
Type      string           `json:"type"`
Text      *TextContent     `json:"text,omitempty"`
Image     *ImageContent    `json:"image,omitempty"`
Document  *DocumentContent `json:"document,omitempty"`
Audio     *AudioContent    `json:"audio,omitempty"`
Video     *VideoContent    `json:"video,omitempty"`
Sticker   *StickerContent  `json:"sticker,omitempty"`
Location  *LocationContent `json:"location,omitempty"`
}

// TextContent holds a text message body.
type TextContent struct {
Body string `json:"body"`
}

// ImageContent holds image message data.
type ImageContent struct {
ID       string `json:"id"`
MimeType string `json:"mime_type"`
SHA256   string `json:"sha256,omitempty"`
Caption  string `json:"caption,omitempty"`
}

// DocumentContent holds document message data.
type DocumentContent struct {
ID       string `json:"id"`
MimeType string `json:"mime_type"`
SHA256   string `json:"sha256,omitempty"`
Filename string `json:"filename,omitempty"`
Caption  string `json:"caption,omitempty"`
}

// AudioContent holds audio message data.
type AudioContent struct {
ID       string `json:"id"`
MimeType string `json:"mime_type"`
SHA256   string `json:"sha256,omitempty"`
}

// VideoContent holds video message data.
type VideoContent struct {
ID       string `json:"id"`
MimeType string `json:"mime_type"`
SHA256   string `json:"sha256,omitempty"`
Caption  string `json:"caption,omitempty"`
}

// StickerContent holds sticker message data.
type StickerContent struct {
ID       string `json:"id"`
MimeType string `json:"mime_type"`
SHA256   string `json:"sha256,omitempty"`
}

// LocationContent holds location message data.
type LocationContent struct {
Latitude  float64 `json:"latitude"`
Longitude float64 `json:"longitude"`
Name      string  `json:"name,omitempty"`
Address   string  `json:"address,omitempty"`
}

// Status represents a message delivery status update.
type Status struct {
ID        string `json:"id"`
Status    string `json:"status"`
Timestamp string `json:"timestamp"`
RecipientID string `json:"recipient_id"`
}

// MediaResponse is the response when retrieving media metadata from the API.
type MediaResponse struct {
URL      string `json:"url"`
MimeType string `json:"mime_type"`
SHA256   string `json:"sha256"`
FileSize int64  `json:"file_size"`
ID       string `json:"id"`
}

// SendMessageRequest is the payload for sending a text message via Kapso.
type SendMessageRequest struct {
MessagingProduct string      `json:"messaging_product"`
RecipientType    string      `json:"recipient_type"`
To               string      `json:"to"`
Type             string      `json:"type"`
Text             TextContent `json:"text"`
}

// SendMessageResponse is the response from the send message API.
type SendMessageResponse struct {
MessagingProduct string `json:"messaging_product"`
Contacts         []struct {
Input string `json:"input"`
WaID  string `json:"wa_id"`
} `json:"contacts"`
Messages []struct {
ID string `json:"id"`
} `json:"messages"`
}
