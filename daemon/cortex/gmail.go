package cortex

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"scaffold/capture"
	"scaffold/db"
	googlemail "scaffold/google"
	"scaffold/sessionbus"
)

func (c *Cortex) runGmailTriage(ctx context.Context) error {
	if c.gmailClient == nil {
		log.Printf("cortex: gmail_triage skipped (gmail client not configured)")
		return nil
	}
	if c.gmailCfg == nil {
		log.Printf("cortex: gmail_triage skipped (gmail config not loaded)")
		return nil
	}

	maxPerRun := 50
	if cfg, ok := c.cfg.Tasks["gmail_triage"]; ok && cfg.MaxPerRun > 0 {
		maxPerRun = cfg.MaxPerRun
	}

	messages, err := c.gmailClient.ListUnread(ctx, maxPerRun)
	if err != nil {
		return fmt.Errorf("list unread: %w", err)
	}
	if len(messages) == 0 {
		log.Printf("cortex: gmail_triage: no unread messages")
		return nil
	}

	labelMap, err := c.gmailClient.ListLabels(ctx)
	if err != nil {
		return fmt.Errorf("list labels: %w", err)
	}

	gmailCfg := c.activeGmailConfig()
	processed := 0

	for _, msg := range messages {
		if hasStatusLabel(msg.Labels, gmailCfg.Labels.Status, labelMap) {
			continue
		}

		var decision *googlemail.TriageDecision

		if d, matched := googlemail.PreFilter(msg, gmailCfg.Prefilter); matched {
			decision = d
		} else {
			d, err := googlemail.LLMTriage(ctx, msg, gmailCfg, c.llm, c.cfg.Bulletin.Model)
			if err != nil {
				log.Printf("cortex: gmail_triage: llm failed for %s (%s): %v", msg.ID, msg.Subject, err)
				continue
			}
			decision = d
		}

		if err := c.applyGmailDecision(ctx, msg, decision, labelMap); err != nil {
			log.Printf("cortex: gmail_triage: apply failed for %s: %v", msg.ID, err)
			continue
		}
		processed++
	}

	log.Printf("cortex: gmail_triage processed=%d/%d", processed, len(messages))
	return nil
}

func (c *Cortex) applyGmailDecision(ctx context.Context, msg googlemail.GmailMessage, decision *googlemail.TriageDecision, labelMap map[string]string) error {
	addLabels := make([]string, 0)

	if decision.StatusLabel != "" && decision.StatusLabel != "none" {
		if id, ok := labelMap[decision.StatusLabel]; ok {
			addLabels = append(addLabels, id)
		} else {
			log.Printf("cortex: gmail_triage: label %q not found in Gmail", decision.StatusLabel)
		}
	}

	if decision.DomainLabel != "" && decision.DomainLabel != "none" {
		if id, ok := labelMap[decision.DomainLabel]; ok {
			addLabels = append(addLabels, id)
		} else {
			log.Printf("cortex: gmail_triage: label %q not found in Gmail", decision.DomainLabel)
		}
	}

	if len(addLabels) > 0 {
		if err := c.gmailClient.ApplyLabels(ctx, msg.ID, addLabels, nil); err != nil {
			return fmt.Errorf("apply labels: %w", err)
		}
	}

	switch strings.ToLower(decision.Action) {
	case "archive":
		if err := c.gmailClient.ArchiveMessage(ctx, msg.ID); err != nil {
			return fmt.Errorf("archive: %w", err)
		}
	case "trash":
		if err := c.gmailClient.TrashMessage(ctx, msg.ID); err != nil {
			return fmt.Errorf("trash: %w", err)
		}
	}

	if decision.CreateTask && strings.TrimSpace(decision.TaskTitle) != "" && c.db != nil {
		task := db.Task{
			ID:       uuid.New().String(),
			Title:    decision.TaskTitle,
			Priority: "normal",
			Source:   sql.NullString{String: "gmail", Valid: true},
			SourceRef: sql.NullString{
				String: fmt.Sprintf("https://mail.google.com/mail/u/0/#inbox/%s", msg.ThreadID),
				Valid:  true,
			},
		}
		if strings.TrimSpace(decision.TaskContext) != "" {
			task.Context = sql.NullString{String: decision.TaskContext, Valid: true}
		}
		if err := c.db.InsertTask(task); err != nil {
			log.Printf("cortex: gmail_triage: create task for %s: %v", msg.ID, err)
		}
	}

	// Low-confidence: surface to inbox for manual review
	if decision.Confidence < 0.6 && c.brain != nil {
		summary := fmt.Sprintf("Low-confidence email triage (%s):\nFrom: %s\nSubject: %s\nProposed: status=%s domain=%s action=%s\n",
			decision.Source, msg.From, msg.Subject, decision.StatusLabel, decision.DomainLabel, decision.Action)
		if _, _, _, err := capture.Ingest(ctx, c.db, c.brain, summary, "gmail-triage"); err != nil {
			log.Printf("cortex: gmail_triage: capture low-confidence: %v", err)
		}
	}

	// WAITING thread tracking
	if strings.EqualFold(decision.StatusLabel, "WAITING") && c.db != nil {
		threadMsgs, err := c.gmailClient.GetThread(ctx, msg.ThreadID)
		if err != nil {
			log.Printf("cortex: gmail_triage: get thread for waiting: %v", err)
		} else {
			if err := c.db.SaveWaitingThread(msg.ThreadID, msg.Subject, nil, "", len(threadMsgs)); err != nil {
				log.Printf("cortex: gmail_triage: save waiting thread: %v", err)
			}
		}
	}

	return nil
}

func (c *Cortex) runWaitingCheck(ctx context.Context) error {
	if c.gmailClient == nil {
		return nil
	}
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}

	threads, err := c.db.GetWaitingThreads()
	if err != nil {
		return fmt.Errorf("get waiting threads: %w", err)
	}
	if len(threads) == 0 {
		return nil
	}

	for _, tracked := range threads {
		msgs, err := c.gmailClient.GetThread(ctx, tracked.ThreadID)
		if err != nil {
			log.Printf("cortex: waiting_check: get thread %s: %v", tracked.ThreadID, err)
			continue
		}

		if len(msgs) <= tracked.MsgCount {
			continue
		}

		newMsg := msgs[len(msgs)-1]
		if !isInboundReply(newMsg) {
			if err := c.db.UpdateWaitingThreadMsgCount(tracked.ThreadID, len(msgs)); err != nil {
				log.Printf("cortex: waiting_check: update msg_count for %s: %v", tracked.ThreadID, err)
			}
			continue
		}
		notification := fmt.Sprintf("Reply received on WAITING thread:\nSubject: %s\nFrom: %s\nSnippet: %s",
			tracked.Subject, newMsg.From, newMsg.Snippet)

		if c.sessionBus != nil {
			_, _ = c.sessionBus.Send(ctx, sessionbus.SendRequest{
				FromSessionID: "cortex",
				ToSessionID:   "scaffold-agent",
				Message:       notification,
			})
		}

		// Create FOLLOW UP task
		if c.db != nil {
			task := db.Task{
				ID:       uuid.New().String(),
				Title:    fmt.Sprintf("Follow up: %s", tracked.Subject),
				Priority: "normal",
				Source:   sql.NullString{String: "gmail", Valid: true},
				SourceRef: sql.NullString{
					String: fmt.Sprintf("https://mail.google.com/mail/u/0/#inbox/%s", tracked.ThreadID),
					Valid:  true,
				},
				Context: sql.NullString{
					String: fmt.Sprintf("Reply received from %s on thread: %s", newMsg.From, tracked.Subject),
					Valid:  true,
				},
			}
			if err := c.db.InsertTask(task); err != nil {
				log.Printf("cortex: waiting_check: create follow-up task: %v", err)
			}
		}

		if err := c.db.DeleteWaitingThread(tracked.ThreadID); err != nil {
			log.Printf("cortex: waiting_check: delete thread: %v", err)
		}

		log.Printf("cortex: waiting_check: reply on %q from %s → notified + task created", tracked.Subject, newMsg.From)
	}

	return nil
}

func hasStatusLabel(messageLabels []string, statusLabels []string, labelMap map[string]string) bool {
	if len(messageLabels) == 0 || len(statusLabels) == 0 {
		return false
	}

	statusNames := make(map[string]struct{}, len(statusLabels))
	statusIDs := make(map[string]struct{}, len(statusLabels))
	for _, status := range statusLabels {
		trimmed := strings.TrimSpace(status)
		if trimmed == "" {
			continue
		}
		statusNames[strings.ToLower(trimmed)] = struct{}{}
		for labelName, labelID := range labelMap {
			if strings.EqualFold(labelName, trimmed) {
				statusIDs[labelID] = struct{}{}
			}
		}
	}

	for _, label := range messageLabels {
		if _, ok := statusIDs[label]; ok {
			return true
		}
		if _, ok := statusNames[strings.ToLower(label)]; ok {
			return true
		}
	}
	return false
}

func isInboundReply(msg googlemail.GmailMessage) bool {
	return !hasLabelID(msg.Labels, "SENT")
}

func hasLabelID(labels []string, id string) bool {
	for _, label := range labels {
		if strings.EqualFold(label, id) {
			return true
		}
	}
	return false
}

func (c *Cortex) activeGmailConfig() googlemail.GmailConfig {
	if c.gmailCfg != nil {
		return *c.gmailCfg
	}
	return googlemail.GmailConfig{}
}
