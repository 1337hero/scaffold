package webhook

import "net/http"

type Event struct {
	Source    string            // "github", "monday"
	EventType string           // "issue.opened", "pr.merged", "ci.failed"
	Title    string
	Body     string
	URL      string            // html_url for source_ref
	Author   string
	Action   string            // "opened", "closed", "merged"
	Repo     string            // "owner/repo"
	Metadata map[string]string
}

type Extractor interface {
	Verify(secret string, headers http.Header, body []byte) error
	Extract(headers http.Header, body []byte) ([]Event, error)
}
