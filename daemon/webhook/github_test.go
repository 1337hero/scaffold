package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"
)

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func headers(event string, sig ...string) http.Header {
	h := http.Header{}
	if event != "" {
		h.Set("X-GitHub-Event", event)
	}
	if len(sig) > 0 {
		h.Set("X-Hub-Signature-256", sig[0])
	}
	return h
}

func TestVerifyValidHMAC(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{"action":"opened"}`)
	secret := "test-secret"

	if err := ext.Verify(secret, headers("issues", sign(secret, body)), body); err != nil {
		t.Fatalf("expected valid HMAC, got: %v", err)
	}
}

func TestVerifyInvalidHMAC(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{"action":"opened"}`)

	err := ext.Verify("real-secret", headers("issues", sign("wrong-secret", body)), body)
	if err == nil {
		t.Fatal("expected HMAC mismatch error")
	}
}

func TestVerifyMissingSignature(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{"action":"opened"}`)

	err := ext.Verify("some-secret", headers("issues"), body)
	if err == nil {
		t.Fatal("expected error for missing signature")
	}
}

func TestVerifyNoSecret(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{"action":"opened"}`)

	if err := ext.Verify("", headers("issues"), body); err != nil {
		t.Fatalf("expected no error when secret is empty, got: %v", err)
	}
}

func TestExtractIssueOpened(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "opened",
		"issue": {"title": "Bug report", "body": "Steps to reproduce...", "html_url": "https://github.com/o/r/issues/1", "number": 1, "user": {"login": "alice"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("issues"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.EventType != "issue.opened" {
		t.Errorf("expected issue.opened, got %q", e.EventType)
	}
	if e.Repo != "owner/repo" {
		t.Errorf("expected repo owner/repo, got %q", e.Repo)
	}
	if e.Author != "alice" {
		t.Errorf("expected author alice, got %q", e.Author)
	}
	if e.URL != "https://github.com/o/r/issues/1" {
		t.Errorf("unexpected URL: %q", e.URL)
	}
}

func TestExtractIssueClosed(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "closed",
		"issue": {"title": "Done", "body": "", "html_url": "https://github.com/o/r/issues/2", "number": 2, "user": {"login": "bob"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("issues"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "issue.closed" {
		t.Errorf("expected issue.closed, got %q", events[0].EventType)
	}
}

func TestExtractIssueIgnoredAction(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "labeled",
		"issue": {"title": "X", "body": "", "html_url": "", "number": 1, "user": {"login": "u"}},
		"repository": {"full_name": "o/r"}
	}`)

	events, err := ext.Extract(headers("issues"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events for labeled action, got %d", len(events))
	}
}

func TestExtractIssueComment(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "created",
		"issue": {"title": "Bug", "html_url": "", "number": 5, "user": {"login": "x"}},
		"comment": {"body": "I can reproduce this", "html_url": "https://github.com/o/r/issues/5#comment-1", "user": {"login": "carol"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("issue_comment"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "issue.comment" {
		t.Errorf("expected issue.comment, got %q", events[0].EventType)
	}
	if events[0].Author != "carol" {
		t.Errorf("expected author carol, got %q", events[0].Author)
	}
}

func TestExtractPROpened(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "opened",
		"pull_request": {"title": "Add feature", "body": "Details", "html_url": "https://github.com/o/r/pull/10", "number": 10, "merged": false, "user": {"login": "dev"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("pull_request"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "pr.opened" {
		t.Errorf("expected pr.opened, got %q", events[0].EventType)
	}
}

func TestExtractPRMerged(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "closed",
		"pull_request": {"title": "Ship it", "body": "", "html_url": "https://github.com/o/r/pull/11", "number": 11, "merged": true, "user": {"login": "dev"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("pull_request"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "pr.merged" {
		t.Errorf("expected pr.merged, got %q", events[0].EventType)
	}
}

func TestExtractPRReviewRequested(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "review_requested",
		"pull_request": {"title": "Need review", "body": "", "html_url": "https://github.com/o/r/pull/12", "number": 12, "merged": false, "user": {"login": "dev"}},
		"requested_reviewer": {"login": "reviewer"},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("pull_request"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "pr.review_requested" {
		t.Errorf("expected pr.review_requested, got %q", events[0].EventType)
	}
}

func TestExtractPush(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"ref": "refs/heads/main",
		"commits": [
			{"message": "fix: bug\ndetails here"},
			{"message": "feat: new thing"}
		],
		"pusher": {"name": "mike"},
		"compare": "https://github.com/o/r/compare/abc...def",
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("push"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.EventType != "push" {
		t.Errorf("expected push, got %q", e.EventType)
	}
	if e.Title != "[owner/repo] Push to main: 2 commits" {
		t.Errorf("unexpected title: %q", e.Title)
	}
}

func TestExtractWorkflowRunFailed(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "completed",
		"workflow_run": {
			"name": "CI",
			"conclusion": "failure",
			"html_url": "https://github.com/o/r/actions/runs/123",
			"head_branch": "main",
			"actor": {"login": "mike"}
		},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("workflow_run"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "ci.failed" {
		t.Errorf("expected ci.failed, got %q", events[0].EventType)
	}
}

func TestExtractWorkflowRunSuccess(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "completed",
		"workflow_run": {"name": "CI", "conclusion": "success", "html_url": "", "head_branch": "main", "actor": {"login": "mike"}},
		"repository": {"full_name": "o/r"}
	}`)

	events, err := ext.Extract(headers("workflow_run"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events for success, got %d", len(events))
	}
}

func TestExtractDiscussion(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "created",
		"discussion": {"title": "RFC: new feature", "body": "Proposal...", "html_url": "https://github.com/o/r/discussions/1", "user": {"login": "mike"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("discussion"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "discussion.created" {
		t.Errorf("expected discussion.created, got %q", events[0].EventType)
	}
}

func TestExtractDiscussionComment(t *testing.T) {
	ext := &GitHubExtractor{}
	body := []byte(`{
		"action": "created",
		"discussion": {"title": "RFC", "html_url": "https://github.com/o/r/discussions/1"},
		"comment": {"body": "Good idea!", "html_url": "https://github.com/o/r/discussions/1#comment-5", "user": {"login": "bob"}},
		"repository": {"full_name": "owner/repo"}
	}`)

	events, err := ext.Extract(headers("discussion_comment"), body)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].EventType != "discussion.comment" {
		t.Errorf("expected discussion.comment, got %q", events[0].EventType)
	}
}

func TestExtractUnknownEvent(t *testing.T) {
	ext := &GitHubExtractor{}
	events, err := ext.Extract(headers("star"), []byte(`{"action":"created"}`))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events for unknown type, got %d", len(events))
	}
}

func TestExtractNoEventHeader(t *testing.T) {
	ext := &GitHubExtractor{}
	events, err := ext.Extract(headers(""), []byte(`{}`))
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events for no event header, got %v", events)
	}
}

func TestExtractMalformedJSON(t *testing.T) {
	ext := &GitHubExtractor{}
	_, err := ext.Extract(headers("issues"), []byte(`not json`))
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
