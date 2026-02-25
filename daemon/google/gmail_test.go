package google

import (
	"encoding/base64"
	"testing"

	"google.golang.org/api/gmail/v1"
)

func TestExtractPlainTextDecodesUnpaddedBase64URL(t *testing.T) {
	text := "hello gmail body"
	encoded := base64.RawURLEncoding.EncodeToString([]byte(text))
	payload := &gmail.MessagePart{
		MimeType: "text/plain",
		Body: &gmail.MessagePartBody{
			Data: encoded,
		},
	}

	got := extractPlainText(payload, 500)
	if got != text {
		t.Fatalf("extractPlainText = %q, want %q", got, text)
	}
}
