package cortex

import (
	"testing"

	googlemail "scaffold/google"
)

func TestParseStatusTag(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"The email requires follow-up. [FOLLOW_UP]", "FOLLOW UP"},
		{"Waiting for reply. [WAITING]", "WAITING"},
		{"Read when you have time. [READ_THROUGH]", "READ THROUGH"},
		{"Not important. [ARCHIVE]", ""},
		{"No tag here", ""},
	}
	for _, tt := range tests {
		got := parseStatusTag(tt.input)
		if got != tt.want {
			t.Errorf("parseStatusTag(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestResolveLabels(t *testing.T) {
	labelMap := map[string]string{
		"FOLLOW UP": "Label_123",
		"INBOX":     "INBOX",
	}
	names := resolveLabels([]string{"INBOX", "Label_123", "Unknown_999"}, labelMap)
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	if names[0] != "INBOX" {
		t.Errorf("expected INBOX, got %s", names[0])
	}
	if names[1] != "FOLLOW UP" {
		t.Errorf("expected FOLLOW UP, got %s", names[1])
	}
	if names[2] != "Unknown_999" {
		t.Errorf("expected raw ID Unknown_999, got %s", names[2])
	}
}

func TestIsInboundReplySkipsSentMessages(t *testing.T) {
	if isInboundReply(googlemail.GmailMessage{Labels: []string{"INBOX", "SENT"}}) {
		t.Fatal("expected SENT message to be treated as outbound")
	}
	if !isInboundReply(googlemail.GmailMessage{Labels: []string{"INBOX", "UNREAD"}}) {
		t.Fatal("expected non-SENT message to be treated as inbound")
	}
}
