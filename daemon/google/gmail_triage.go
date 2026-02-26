package google

import (
	"regexp"
	"strings"
)

type GmailConfig struct {
	Prefilter     PrefilterConfig   `yaml:"prefilter"`
	Labels        LabelConfig       `yaml:"labels"`
	SystemLabel   string            `yaml:"system_label"`
	MarkRead      *bool             `yaml:"mark_read"`
	DomainMapping map[string]string `yaml:"domain_mapping"`
}

type PrefilterConfig struct {
	KnownFilers        []KnownFilerRule `yaml:"known_filers"`
	GithubActionsTrash bool             `yaml:"github_actions_trash"`
	NeverTrash         []string         `yaml:"never_trash"`
}

type KnownFilerRule struct {
	Pattern     string `yaml:"pattern"`
	Match       string `yaml:"match"` // "subject" or "from"
	DomainLabel string `yaml:"domain_label"`
	Action      string `yaml:"action"` // "archive" | "trash"
}

type LabelConfig struct {
	Status []string `yaml:"status"`
	Domain []string `yaml:"domain"`
}

type PreFilterResult struct {
	DomainLabel string
	Action      string // "archive" | "trash"
}

func matchesNeverTrash(from string, neverTrash []string) bool {
	lower := strings.ToLower(from)
	for _, pattern := range neverTrash {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return true
		}
	}
	return false
}

// PreFilter applies deterministic rules. Returns (result, matched).
func PreFilter(msg GmailMessage, cfg PrefilterConfig) (*PreFilterResult, bool) {
	// Protected senders: skip prefilter entirely
	if matchesNeverTrash(msg.From, cfg.NeverTrash) {
		return nil, false
	}

	// Rule 1: known filer patterns — explicit user rules take priority over heuristics
	for _, rule := range cfg.KnownFilers {
		pat, err := regexp.Compile("(?i)" + rule.Pattern)
		if err != nil {
			continue
		}
		var target string
		switch strings.ToLower(rule.Match) {
		case "from":
			target = msg.From
		default:
			target = msg.Subject
		}
		if pat.MatchString(target) {
			return &PreFilterResult{
				DomainLabel: rule.DomainLabel,
				Action:      rule.Action,
			}, true
		}
	}

	// Rule 2: marketing/unsubscribe (after known filers so explicit rules win)
	if msg.HasUnsubscribe || msg.GmailCategory == "CATEGORY_PROMOTIONS" {
		return &PreFilterResult{Action: "trash"}, true
	}

	// Rule 3: GitHub Actions
	if cfg.GithubActionsTrash && isGithubActionsBot(msg.From) {
		return &PreFilterResult{Action: "trash"}, true
	}

	return nil, false
}

func isGithubActionsBot(from string) bool {
	lower := strings.ToLower(from)
	return strings.Contains(lower, "github-actions") ||
		strings.Contains(lower, "noreply@github.com") ||
		strings.Contains(lower, "notifications@github.com")
}

