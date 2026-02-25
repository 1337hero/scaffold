package google

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"scaffold/llm"
)

type GmailConfig struct {
	Prefilter PrefilterConfig `yaml:"prefilter"`
	Labels    LabelConfig     `yaml:"labels"`
}

type PrefilterConfig struct {
	KnownFilers        []KnownFilerRule `yaml:"known_filers"`
	GithubActionsTrash bool             `yaml:"github_actions_trash"`
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

type TriageDecision struct {
	StatusLabel string  `json:"status_label"`
	DomainLabel string  `json:"domain_label"`
	Action      string  `json:"action"` // keep | archive | trash
	CreateTask  bool    `json:"create_task"`
	TaskTitle   string  `json:"task_title"`
	TaskContext string  `json:"task_context"`
	Confidence  float64 `json:"confidence"`
	Source      string  `json:"-"` // "prefilter" | "llm"
}

// PreFilter applies deterministic rules before LLM. Returns (decision, matched).
func PreFilter(msg GmailMessage, cfg PrefilterConfig) (*TriageDecision, bool) {
	// Rule 1: marketing
	if msg.HasUnsubscribe || msg.GmailCategory == "CATEGORY_PROMOTIONS" {
		return &TriageDecision{Action: "trash", Source: "prefilter", Confidence: 1.0}, true
	}

	// Rule 2: GitHub Actions — trash if configured (webhook dedup already handles this)
	if cfg.GithubActionsTrash && isGithubActionsBot(msg.From) {
		return &TriageDecision{Action: "trash", Source: "prefilter", Confidence: 0.9}, true
	}

	// Rule 3: known filer patterns
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
			return &TriageDecision{
				DomainLabel: rule.DomainLabel,
				Action:      rule.Action,
				Source:      "prefilter",
				Confidence:  1.0,
			}, true
		}
	}

	return nil, false
}

func isGithubActionsBot(from string) bool {
	lower := strings.ToLower(from)
	return strings.Contains(lower, "github-actions") ||
		strings.Contains(lower, "noreply@github.com") ||
		strings.Contains(lower, "notifications@github.com")
}

// LLMTriage calls the LLM to classify an email and returns a structured decision.
func LLMTriage(ctx context.Context, msg GmailMessage, cfg GmailConfig, client llm.CompletionClient, model string) (*TriageDecision, error) {
	domainList := strings.Join(cfg.Labels.Domain, "\n- ")
	system := fmt.Sprintf(`You are an email triage agent. Classify emails using a two-axis label model and return structured JSON.

Status axis (what to do — email stays in inbox):
- FOLLOW UP: email requires action from me
- WAITING: I sent something and am waiting for a response
- READ THROUGH: needs careful reading but no action
- STAND BY: tracking something but not urgent
- none: no status label needed

Domain axis (what it is — for filing):
- %s
- none: no domain label

Actions:
- keep: email stays in inbox (required if status_label is set and not "none")
- archive: move out of inbox
- trash: delete

Rules:
- If status_label is set and not "none", action must be "keep"
- If domain_label only (no status), action should be "archive"
- If clearly actionable, set create_task=true with a clear task_title
- Confidence < 0.6 means you are uncertain

Respond ONLY with valid JSON, no other text.`, domainList)

	user := fmt.Sprintf(`Email to classify:
From: %s
Subject: %s
Date: %s
Gmail Labels: %s
Snippet: %s

Body preview:
%s

Return JSON: {"status_label": "...", "domain_label": "...", "action": "keep|archive|trash", "create_task": false, "task_title": "", "task_context": "", "confidence": 0.0}`,
		msg.From, msg.Subject, msg.Date.Format("2006-01-02 15:04"),
		strings.Join(msg.Labels, ", "),
		msg.Snippet, msg.Body)

	raw, err := client.CompletionJSON(ctx, model, system, user, 300)
	if err != nil {
		return nil, fmt.Errorf("llm triage: %w", err)
	}

	var decision TriageDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return nil, fmt.Errorf("parse triage response: %w (raw: %s)", err, raw)
	}
	decision.Source = "llm"
	return &decision, nil
}
