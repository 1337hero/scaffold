package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

func init() {
	Register("github", &GitHubExtractor{})
}

type GitHubExtractor struct{}

func (g *GitHubExtractor) Verify(secret string, headers http.Header, body []byte) error {
	if secret == "" {
		return nil
	}
	sig := headers.Get("X-Hub-Signature-256")
	if sig == "" {
		return fmt.Errorf("missing X-Hub-Signature-256 header")
	}
	prefix := "sha256="
	if !strings.HasPrefix(sig, prefix) {
		return fmt.Errorf("invalid signature format")
	}
	got, err := hex.DecodeString(sig[len(prefix):])
	if err != nil {
		return fmt.Errorf("invalid signature hex: %w", err)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)

	if !hmac.Equal(got, expected) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

func (g *GitHubExtractor) Extract(headers http.Header, body []byte) ([]Event, error) {
	eventType := headers.Get("X-GitHub-Event")
	if eventType == "" {
		return nil, nil
	}

	switch eventType {
	case "issues":
		return g.extractIssue(body)
	case "issue_comment":
		return g.extractIssueComment(body)
	case "pull_request":
		return g.extractPR(body)
	case "push":
		return g.extractPush(body)
	case "workflow_run":
		return g.extractWorkflowRun(body)
	case "discussion":
		return g.extractDiscussion(body)
	case "discussion_comment":
		return g.extractDiscussionComment(body)
	default:
		return nil, nil
	}
}

type ghRepo struct {
	FullName string `json:"full_name"`
}

type ghUser struct {
	Login string `json:"login"`
}

type ghIssue struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Number  int    `json:"number"`
	User    ghUser `json:"user"`
}

type ghPR struct {
	Title   string `json:"title"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
	Number  int    `json:"number"`
	Merged  bool   `json:"merged"`
	User    ghUser `json:"user"`
}

func (g *GitHubExtractor) extractIssue(body []byte) ([]Event, error) {
	var payload struct {
		Action string  `json:"action"`
		Issue  ghIssue `json:"issue"`
		Repo   ghRepo  `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse issue event: %w", err)
	}

	var eventType, title string
	switch payload.Action {
	case "opened":
		eventType = "issue.opened"
		title = fmt.Sprintf("[%s] Issue opened: %s", payload.Repo.FullName, payload.Issue.Title)
	case "closed":
		eventType = "issue.closed"
		title = fmt.Sprintf("[%s] Issue closed: %s", payload.Repo.FullName, payload.Issue.Title)
	default:
		return nil, nil
	}

	return []Event{{
		Source:    "github",
		EventType: eventType,
		Title:    title,
		Body:     truncate(payload.Issue.Body, 2000),
		URL:      payload.Issue.HTMLURL,
		Author:   payload.Issue.User.Login,
		Action:   payload.Action,
		Repo:     payload.Repo.FullName,
	}}, nil
}

func (g *GitHubExtractor) extractIssueComment(body []byte) ([]Event, error) {
	var payload struct {
		Action  string `json:"action"`
		Issue   ghIssue `json:"issue"`
		Comment struct {
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    ghUser `json:"user"`
		} `json:"comment"`
		Repo ghRepo `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse issue_comment event: %w", err)
	}
	if payload.Action != "created" {
		return nil, nil
	}

	return []Event{{
		Source:    "github",
		EventType: "issue.comment",
		Title:    fmt.Sprintf("[%s] Comment on #%d", payload.Repo.FullName, payload.Issue.Number),
		Body:     truncate(payload.Comment.Body, 2000),
		URL:      payload.Comment.HTMLURL,
		Author:   payload.Comment.User.Login,
		Action:   "created",
		Repo:     payload.Repo.FullName,
	}}, nil
}

func (g *GitHubExtractor) extractPR(body []byte) ([]Event, error) {
	var payload struct {
		Action      string `json:"action"`
		PullRequest ghPR   `json:"pull_request"`
		Repo        ghRepo `json:"repository"`
		RequestedReviewer *ghUser `json:"requested_reviewer"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse pull_request event: %w", err)
	}

	pr := payload.PullRequest
	var eventType, title string

	switch {
	case payload.Action == "opened":
		eventType = "pr.opened"
		title = fmt.Sprintf("[%s] PR opened: %s", payload.Repo.FullName, pr.Title)
	case payload.Action == "closed" && pr.Merged:
		eventType = "pr.merged"
		title = fmt.Sprintf("[%s] PR merged: %s", payload.Repo.FullName, pr.Title)
	case payload.Action == "review_requested":
		eventType = "pr.review_requested"
		title = fmt.Sprintf("[%s] Review requested: %s", payload.Repo.FullName, pr.Title)
	default:
		return nil, nil
	}

	return []Event{{
		Source:    "github",
		EventType: eventType,
		Title:    title,
		Body:     truncate(pr.Body, 2000),
		URL:      pr.HTMLURL,
		Author:   pr.User.Login,
		Action:   payload.Action,
		Repo:     payload.Repo.FullName,
	}}, nil
}

func (g *GitHubExtractor) extractPush(body []byte) ([]Event, error) {
	var payload struct {
		Ref     string `json:"ref"`
		Commits []struct {
			Message string `json:"message"`
		} `json:"commits"`
		Pusher struct {
			Name string `json:"name"`
		} `json:"pusher"`
		Compare string `json:"compare"`
		Repo    ghRepo `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse push event: %w", err)
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	n := len(payload.Commits)

	var bodyText string
	for i, c := range payload.Commits {
		if i >= 5 {
			bodyText += fmt.Sprintf("... and %d more commits", n-5)
			break
		}
		msg := c.Message
		if idx := strings.IndexByte(msg, '\n'); idx > 0 {
			msg = msg[:idx]
		}
		bodyText += "- " + msg + "\n"
	}

	return []Event{{
		Source:    "github",
		EventType: "push",
		Title:    fmt.Sprintf("[%s] Push to %s: %d commits", payload.Repo.FullName, branch, n),
		Body:     bodyText,
		URL:      payload.Compare,
		Author:   payload.Pusher.Name,
		Action:   "push",
		Repo:     payload.Repo.FullName,
	}}, nil
}

func (g *GitHubExtractor) extractWorkflowRun(body []byte) ([]Event, error) {
	var payload struct {
		Action      string `json:"action"`
		WorkflowRun struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
			HeadBranch string `json:"head_branch"`
			Actor      ghUser `json:"actor"`
		} `json:"workflow_run"`
		Repo ghRepo `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse workflow_run event: %w", err)
	}

	if payload.Action != "completed" || payload.WorkflowRun.Conclusion != "failure" {
		return nil, nil
	}

	wr := payload.WorkflowRun
	return []Event{{
		Source:    "github",
		EventType: "ci.failed",
		Title:    fmt.Sprintf("[%s] CI failed: %s on %s", payload.Repo.FullName, wr.Name, wr.HeadBranch),
		Body:     fmt.Sprintf("Workflow %q failed on branch %s", wr.Name, wr.HeadBranch),
		URL:      wr.HTMLURL,
		Author:   wr.Actor.Login,
		Action:   "failed",
		Repo:     payload.Repo.FullName,
	}}, nil
}

func (g *GitHubExtractor) extractDiscussion(body []byte) ([]Event, error) {
	var payload struct {
		Action     string `json:"action"`
		Discussion struct {
			Title   string `json:"title"`
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    ghUser `json:"user"`
		} `json:"discussion"`
		Repo ghRepo `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse discussion event: %w", err)
	}
	if payload.Action != "created" {
		return nil, nil
	}

	d := payload.Discussion
	return []Event{{
		Source:    "github",
		EventType: "discussion.created",
		Title:    fmt.Sprintf("[%s] Discussion: %s", payload.Repo.FullName, d.Title),
		Body:     truncate(d.Body, 2000),
		URL:      d.HTMLURL,
		Author:   d.User.Login,
		Action:   "created",
		Repo:     payload.Repo.FullName,
	}}, nil
}

func (g *GitHubExtractor) extractDiscussionComment(body []byte) ([]Event, error) {
	var payload struct {
		Action     string `json:"action"`
		Discussion struct {
			Title   string `json:"title"`
			HTMLURL string `json:"html_url"`
		} `json:"discussion"`
		Comment struct {
			Body    string `json:"body"`
			HTMLURL string `json:"html_url"`
			User    ghUser `json:"user"`
		} `json:"comment"`
		Repo ghRepo `json:"repository"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse discussion_comment event: %w", err)
	}
	if payload.Action != "created" {
		return nil, nil
	}

	return []Event{{
		Source:    "github",
		EventType: "discussion.comment",
		Title:    fmt.Sprintf("[%s] Discussion comment on: %s", payload.Repo.FullName, payload.Discussion.Title),
		Body:     truncate(payload.Comment.Body, 2000),
		URL:      payload.Comment.HTMLURL,
		Author:   payload.Comment.User.Login,
		Action:   "created",
		Repo:     payload.Repo.FullName,
	}}, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
