package cortex

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"scaffold/capture"
	appconfig "scaffold/config"
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
	if c.brain == nil {
		log.Printf("cortex: gmail_triage skipped (brain not configured)")
		return nil
	}

	maxPerRun := 50
	if cfg, ok := c.cfg.Tasks["gmail_triage"]; ok && cfg.MaxPerRun > 0 {
		maxPerRun = cfg.MaxPerRun
	}

	messages, err := c.gmailClient.ListInbox(ctx, maxPerRun)
	if err != nil {
		return fmt.Errorf("list inbox: %w", err)
	}
	if len(messages) == 0 {
		log.Printf("cortex: gmail_triage: inbox empty")
		return nil
	}

	labelMap, err := c.gmailClient.ListLabels(ctx)
	if err != nil {
		return fmt.Errorf("list labels: %w", err)
	}
	systemLabelID := ""
	if c.gmailCfg.SystemLabel != "" {
		systemLabelID = labelMap[c.gmailCfg.SystemLabel]
	}

	emailRules := loadEmailRules(c.cfg)

	processed, prefiltered, brainRouted := 0, 0, 0

	for _, msg := range messages {
		// Skip already-triaged (system label = idempotency marker)
		if systemLabelID != "" && hasLabelID(msg.Labels, systemLabelID) {
			continue
		}

		// PreFilter: deterministic rules
		if result, matched := googlemail.PreFilter(msg, c.gmailCfg.Prefilter); matched {
			if err := c.applyPreFilterResult(ctx, msg, result, labelMap, systemLabelID); err != nil {
				log.Printf("cortex: gmail_triage: prefilter apply failed for %s: %v", msg.ID, err)
			} else {
				prefiltered++
				processed++
			}
			continue
		}

		// Brain triage
		triageMsg := buildTriageMessage(msg, c.gmailCfg, c.gmailClient, c.db, ctx, emailRules, labelMap)

		brainCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
		response, err := c.brain.Respond(brainCtx, triageMsg, nil)
		cancel()

		if err != nil {
			log.Printf("cortex: gmail_triage: brain failed for %s (%s): %v — falling back to capture", msg.ID, msg.Subject, err)
			if c.brain != nil {
				summary := fmt.Sprintf("Email triage failed — manual review needed:\nFrom: %s\nSubject: %s\nLink: %s",
					msg.From, msg.Subject, googlemail.GmailPermalink(msg.ID))
				_, _, _, _ = capture.Ingest(ctx, c.db, c.brain, summary, "gmail-triage")
			}
			// Still label + archive even on failure
			_ = c.applyBrainResult(ctx, msg, "", labelMap, systemLabelID)
			processed++
			continue
		}

		// Log the brain exchange
		if c.db != nil {
			_, _ = c.db.InsertConversationEntry("gmail-triage", "user", triageMsg)
			_, _ = c.db.InsertConversationEntry("gmail-triage", "assistant", response)
		}

		statusLabel := parseStatusTag(response)
		if err := c.applyBrainResult(ctx, msg, statusLabel, labelMap, systemLabelID); err != nil {
			log.Printf("cortex: gmail_triage: brain apply failed for %s: %v", msg.ID, err)
		}

		brainRouted++
		processed++
	}

	log.Printf("cortex: gmail_triage processed=%d prefiltered=%d brain=%d total=%d", processed, prefiltered, brainRouted, len(messages))
	return nil
}

func loadEmailRules(cfg appconfig.CortexConfig) string {
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = "./config"
	}
	path := filepath.Join(configDir, "agent-email.md")
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("cortex: gmail_triage: agent-email.md not found at %s, using minimal rules", path)
		return "Classify this email. Use create_task for actionable items. End with a status tag: [FOLLOW_UP] [WAITING] [READ_THROUGH] [ARCHIVE]"
	}
	return string(data)
}

func buildTriageMessage(msg googlemail.GmailMessage, cfg *googlemail.GmailConfig, gmailClient *googlemail.GmailClient, database *db.DB, ctx context.Context, emailRules string, labelMap map[string]string) string {
	var sb strings.Builder

	sb.WriteString(emailRules)
	sb.WriteString("\n\n---\n\n")

	sb.WriteString("[EMAIL TRIAGE]\n")
	sb.WriteString(fmt.Sprintf("From: %s\n", msg.From))
	sb.WriteString(fmt.Sprintf("Subject: %s\n", msg.Subject))
	sb.WriteString(fmt.Sprintf("Date: %s\n", msg.Date.Format("2006-01-02")))

	labelNames := resolveLabels(msg.Labels, labelMap)
	if len(labelNames) > 0 {
		sb.WriteString(fmt.Sprintf("Gmail Labels: %s\n", strings.Join(labelNames, ", ")))
	}

	if cfg != nil && cfg.DomainMapping != nil {
		for _, labelName := range labelNames {
			if domain, ok := cfg.DomainMapping[labelName]; ok {
				sb.WriteString(fmt.Sprintf("Pre-sorted Domain: %s → Scaffold domain: %s\n", labelName, domain))
				break
			}
		}
	}

	permalink := googlemail.GmailPermalink(msg.ID)
	sb.WriteString(fmt.Sprintf("Gmail Link: %s\n", permalink))

	if len(msg.Attachments) > 0 {
		sb.WriteString(fmt.Sprintf("\nAttachments: %s\n", strings.Join(msg.Attachments, ", ")))
	}

	if msg.Body != "" {
		sb.WriteString(fmt.Sprintf("\nBody preview:\n%s\n", msg.Body))
	}

	if msg.ThreadID != "" && gmailClient != nil {
		threadCtx := getLastHumanMessage(ctx, gmailClient, msg.ThreadID, msg.ID)
		if threadCtx != "" {
			sb.WriteString(fmt.Sprintf("\nThread context: (last human-written message)\n%s\n", threadCtx))
		}
	}

	if database != nil {
		existing, _ := database.TaskBySourceRef(permalink)
		if existing != nil {
			sb.WriteString(fmt.Sprintf("\n---\nDedup check: Task already exists (id=%s, title=%q). Do NOT create a duplicate.\n---\n", existing.ID, existing.Title))
		} else {
			sb.WriteString("\n---\nDedup check: No existing task for this thread.\n---\n")
		}
	}

	sb.WriteString("\nClassify this email. End your response with exactly one tag:\n[FOLLOW_UP] [WAITING] [READ_THROUGH] [ARCHIVE]\n")

	return sb.String()
}

func resolveLabels(labelIDs []string, labelMap map[string]string) []string {
	idToName := make(map[string]string, len(labelMap))
	for name, id := range labelMap {
		idToName[id] = name
	}
	names := make([]string, 0, len(labelIDs))
	for _, id := range labelIDs {
		if name, ok := idToName[id]; ok {
			names = append(names, name)
		} else {
			names = append(names, id)
		}
	}
	return names
}

func getLastHumanMessage(ctx context.Context, client *googlemail.GmailClient, threadID, currentMsgID string) string {
	threadMsgs, err := client.GetThread(ctx, threadID)
	if err != nil || len(threadMsgs) <= 1 {
		return ""
	}
	for i := len(threadMsgs) - 1; i >= 0; i-- {
		m := threadMsgs[i]
		if m.ID == currentMsgID {
			continue
		}
		from := strings.ToLower(m.From)
		if strings.Contains(from, "noreply") || strings.Contains(from, "no-reply") ||
			strings.Contains(from, "mailer-daemon") || strings.Contains(from, "notifications@") {
			continue
		}
		return fmt.Sprintf("[%s] From: %s\n  %s", m.Date.Format("2006-01-02"), m.From, m.Snippet)
	}
	return ""
}

func parseStatusTag(response string) string {
	tags := map[string]string{
		"[FOLLOW_UP]":    "FOLLOW UP",
		"[WAITING]":      "WAITING",
		"[READ_THROUGH]": "READ THROUGH",
		"[ARCHIVE]":      "",
	}
	for tag, label := range tags {
		if strings.Contains(response, tag) {
			return label
		}
	}
	return ""
}

func (c *Cortex) applyPreFilterResult(ctx context.Context, msg googlemail.GmailMessage, result *googlemail.PreFilterResult, labelMap map[string]string, systemLabelID string) error {
	addLabels := make([]string, 0, 2)
	removeLabels := make([]string, 0, 2)

	if result.DomainLabel != "" {
		if id, ok := labelMap[result.DomainLabel]; ok {
			addLabels = append(addLabels, id)
		}
	}

	if systemLabelID != "" {
		addLabels = append(addLabels, systemLabelID)
	}

	markRead := c.gmailCfg == nil || c.gmailCfg.MarkRead == nil || *c.gmailCfg.MarkRead
	if markRead {
		removeLabels = append(removeLabels, "UNREAD")
	}

	if len(addLabels) > 0 || len(removeLabels) > 0 {
		if err := c.gmailClient.ApplyLabels(ctx, msg.ID, addLabels, removeLabels); err != nil {
			return fmt.Errorf("apply labels: %w", err)
		}
	}

	if strings.EqualFold(result.Action, "trash") {
		return c.gmailClient.TrashMessage(ctx, msg.ID)
	}
	return c.gmailClient.ArchiveMessage(ctx, msg.ID)
}

func (c *Cortex) applyBrainResult(ctx context.Context, msg googlemail.GmailMessage, statusLabel string, labelMap map[string]string, systemLabelID string) error {
	addLabels := make([]string, 0, 3)
	removeLabels := make([]string, 0, 2)

	if statusLabel != "" {
		if id, ok := labelMap[statusLabel]; ok {
			addLabels = append(addLabels, id)
		} else {
			log.Printf("cortex: gmail_triage: status label %q not found in Gmail", statusLabel)
		}
	}

	if c.gmailCfg != nil && c.gmailCfg.DomainMapping != nil {
		idToName := make(map[string]string, len(labelMap))
		for name, id := range labelMap {
			idToName[id] = name
		}
		for _, labelID := range msg.Labels {
			name := idToName[labelID]
			if name == "" {
				name = labelID
			}
			if _, ok := c.gmailCfg.DomainMapping[name]; ok {
				if id, ok := labelMap[name]; ok {
					addLabels = append(addLabels, id)
				}
				break
			}
		}
	}

	if systemLabelID != "" {
		addLabels = append(addLabels, systemLabelID)
	}

	markRead := c.gmailCfg == nil || c.gmailCfg.MarkRead == nil || *c.gmailCfg.MarkRead
	if markRead {
		removeLabels = append(removeLabels, "UNREAD")
	}

	if len(addLabels) > 0 || len(removeLabels) > 0 {
		if err := c.gmailClient.ApplyLabels(ctx, msg.ID, addLabels, removeLabels); err != nil {
			return fmt.Errorf("apply labels: %w", err)
		}
	}

	// WAITING thread tracking
	if strings.EqualFold(statusLabel, "WAITING") && c.db != nil {
		threadMsgs, err := c.gmailClient.GetThreadMinimal(ctx, msg.ThreadID)
		if err != nil {
			log.Printf("cortex: gmail_triage: get thread for waiting: %v", err)
		} else {
			lastMsgID := ""
			if len(threadMsgs) > 0 {
				lastMsgID = threadMsgs[len(threadMsgs)-1].ID
			}
			c.db.SaveWaitingThread(msg.ThreadID, msg.Subject, nil, "", len(threadMsgs), lastMsgID)
		}
	}

	return c.gmailClient.ArchiveMessage(ctx, msg.ID)
}

// --- runWaitingCheck (unchanged) ---

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
		msgs, err := c.gmailClient.GetThreadMinimal(ctx, tracked.ThreadID)
		if err != nil {
			log.Printf("cortex: waiting_check: get thread %s: %v", tracked.ThreadID, err)
			continue
		}

		if len(msgs) == 0 {
			continue
		}

		lastMsg := msgs[len(msgs)-1]

		var hasNewMessage bool
		if tracked.LastMessageID != "" {
			hasNewMessage = lastMsg.ID != tracked.LastMessageID
		} else {
			hasNewMessage = len(msgs) > tracked.MsgCount
		}

		if !hasNewMessage {
			continue
		}

		fullMsg, err := c.gmailClient.GetMessage(ctx, lastMsg.ID)
		if err != nil {
			log.Printf("cortex: waiting_check: get new message %s: %v", lastMsg.ID, err)
			_ = c.db.UpdateWaitingThreadLastMessageID(tracked.ThreadID, lastMsg.ID)
			continue
		}

		if !isInboundReply(*fullMsg) {
			if err := c.db.UpdateWaitingThreadLastMessageID(tracked.ThreadID, lastMsg.ID); err != nil {
				log.Printf("cortex: waiting_check: update last_message_id for %s: %v", tracked.ThreadID, err)
			}
			continue
		}

		notification := fmt.Sprintf("Reply received on WAITING thread:\nSubject: %s\nFrom: %s\nSnippet: %s",
			tracked.Subject, fullMsg.From, fullMsg.Snippet)

		if c.sessionBus != nil {
			_, _ = c.sessionBus.Send(ctx, sessionbus.SendRequest{
				FromSessionID: "cortex",
				ToSessionID:   "scaffold-agent",
				Message:       notification,
			})
		}

		if c.db != nil {
			sourceRef := fmt.Sprintf("https://mail.google.com/mail/u/0/#inbox/%s", tracked.ThreadID)
			if existing, _ := c.db.TaskBySourceRef(sourceRef); existing != nil {
				log.Printf("cortex: waiting_check: task already exists for thread %s, skipping", tracked.ThreadID)
			} else {
				task := db.Task{
					ID:        uuid.New().String(),
					Title:     fmt.Sprintf("Follow up: %s", tracked.Subject),
					Priority:  "normal",
					Source:    sql.NullString{String: "gmail", Valid: true},
					SourceRef: sql.NullString{String: sourceRef, Valid: true},
					Context: sql.NullString{
						String: fmt.Sprintf("Reply received from %s on thread: %s", fullMsg.From, tracked.Subject),
						Valid:  true,
					},
				}
				if err := c.db.InsertTask(task); err != nil {
					log.Printf("cortex: waiting_check: create follow-up task: %v", err)
				}
			}
		}

		if err := c.db.DeleteWaitingThread(tracked.ThreadID); err != nil {
			log.Printf("cortex: waiting_check: delete thread: %v", err)
		}

		log.Printf("cortex: waiting_check: reply on %q from %s → notified + task created", tracked.Subject, fullMsg.From)
	}

	return nil
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
