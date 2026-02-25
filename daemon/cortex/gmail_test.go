package cortex

import (
	"testing"

	googlemail "scaffold/google"
)

func TestHasStatusLabelMatchesLabelIDs(t *testing.T) {
	labelMap := map[string]string{
		"FOLLOW UP": "Label_123",
		"WAITING":   "Label_456",
	}
	statusLabels := []string{"FOLLOW UP", "WAITING"}
	messageLabels := []string{"INBOX", "UNREAD", "Label_456"}

	if !hasStatusLabel(messageLabels, statusLabels, labelMap) {
		t.Fatal("expected status label match via Gmail label ID")
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
