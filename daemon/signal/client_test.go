package signal

import (
	"strings"
	"testing"
)

func TestParseInboundValidMessage(t *testing.T) {
	event := SSEEvent{
		Event: "receive",
		Data:  `{"envelope":{"source":"+15551234567","dataMessage":{"message":"hello"}}}`,
	}

	msg := ParseInbound(event)
	if msg == nil {
		t.Fatal("expected inbound message, got nil")
	}
	if msg.Sender != "+15551234567" {
		t.Fatalf("expected sender +15551234567, got %s", msg.Sender)
	}
	if msg.Message != "hello" {
		t.Fatalf("expected message hello, got %s", msg.Message)
	}
	if msg.HasNonTextContent() {
		t.Fatalf("expected no attachments, got %+v", msg.AttachmentKinds)
	}
}

func TestParseInboundNoDataMessage(t *testing.T) {
	event := SSEEvent{
		Event: "receive",
		Data:  `{"envelope":{"source":"+15551234567"}}`,
	}

	if msg := ParseInbound(event); msg != nil {
		t.Fatalf("expected nil message, got %+v", msg)
	}
}

func TestParseInboundInvalidJSON(t *testing.T) {
	event := SSEEvent{
		Event: "receive",
		Data:  `{"envelope":`,
	}

	if msg := ParseInbound(event); msg != nil {
		t.Fatalf("expected nil message, got %+v", msg)
	}
}

func TestParseInboundAttachmentOnly(t *testing.T) {
	event := SSEEvent{
		Event: "receive",
		Data:  `{"envelope":{"source":"+15551234567","dataMessage":{"attachments":[{"contentType":"image/jpeg"}]}}}`,
	}

	msg := ParseInbound(event)
	if msg == nil {
		t.Fatal("expected inbound message, got nil")
	}
	if msg.Message != "" {
		t.Fatalf("expected empty text message, got %q", msg.Message)
	}
	if !msg.HasNonTextContent() {
		t.Fatal("expected non-text content")
	}
	if got := msg.NonTextContentSummary(); got != "image" {
		t.Fatalf("expected image summary, got %q", got)
	}
}

func TestParseInboundAudioAndAttachmentSummary(t *testing.T) {
	event := SSEEvent{
		Event: "receive",
		Data:  `{"envelope":{"source":"+15551234567","dataMessage":{"message":"check this","attachments":[{"contentType":"audio/ogg","voiceNote":true},{"contentType":"application/pdf"}]}}}`,
	}

	msg := ParseInbound(event)
	if msg == nil {
		t.Fatal("expected inbound message, got nil")
	}
	if msg.Message != "check this" {
		t.Fatalf("expected text message, got %q", msg.Message)
	}
	if got := msg.NonTextContentSummary(); got != "audio and attachment" {
		t.Fatalf("expected non-text summary, got %q", got)
	}
}

func TestParseSSEDispatchesEvents(t *testing.T) {
	stream := strings.NewReader("event: receive\ndata: one\n\nevent: receive\ndata: two\n\n")

	var events []SSEEvent
	err := parseSSE(stream, func(event SSEEvent) {
		events = append(events, event)
	})
	if err != nil {
		t.Fatalf("parseSSE returned error: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Data != "one" || events[1].Data != "two" {
		t.Fatalf("unexpected events: %+v", events)
	}
}
