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
