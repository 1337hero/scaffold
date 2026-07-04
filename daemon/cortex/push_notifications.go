package cortex

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"scaffold/db"
	"scaffold/notify"
)

const birthdayCheckSchedule = "08:00"

func (c *Cortex) maybeRunBirthdayCheck(ctx context.Context, now time.Time) {
	if c == nil || !c.notifyCfg.Enabled {
		return
	}
	localNow := now.In(briefLocation())
	if sameBriefMinute(localNow, birthdayCheckSchedule) {
		c.startBirthdayCheck(ctx, localNow)
	}
}

func (c *Cortex) startBirthdayCheck(ctx context.Context, localNow time.Time) {
	if !c.claimBriefRun("birthday_check", localNow) {
		return
	}

	go func() {
		taskCtx, cancel := context.WithTimeout(ctx, dailyBriefTimeout)
		defer cancel()

		start := time.Now()
		if err := c.runBirthdayCheck(taskCtx); err != nil {
			log.Printf("cortex: birthday_check failed (%v): %v", time.Since(start), err)
			return
		}
		log.Printf("cortex: birthday_check completed (%v)", time.Since(start))
	}()
}

func (c *Cortex) runPushCheck(ctx context.Context) error {
	if c == nil || !c.notifyCfg.Enabled {
		return nil
	}
	if c.sendSignal == nil {
		return fmt.Errorf("signal sender is not configured")
	}

	now := time.Now().In(briefLocation())
	notifications, err := c.collectTaskReminderNotifications(now)
	if err != nil {
		return err
	}
	followUps, err := c.collectFollowUpNotifications(now)
	if err != nil {
		return err
	}
	notifications = append(notifications, followUps...)
	return c.sendNotificationBatch(ctx, notifications)
}

func (c *Cortex) runBirthdayCheck(ctx context.Context) error {
	if c == nil || !c.notifyCfg.Enabled {
		return nil
	}
	if c.sendSignal == nil {
		return fmt.Errorf("signal sender is not configured")
	}

	notifications, err := c.collectBirthdayNotifications(time.Now().In(briefLocation()))
	if err != nil {
		return err
	}
	return c.sendNotificationBatch(ctx, notifications)
}

func (c *Cortex) collectTaskReminderNotifications(now time.Time) ([]notify.Notification, error) {
	cutoff := now.UTC().Format(time.RFC3339)
	tasks, err := c.db.ListTasks(db.TaskFilters{Status: "pending", ReminderAt: &cutoff})
	if err != nil {
		return nil, fmt.Errorf("load task reminders: %w", err)
	}

	out := make([]notify.Notification, 0, len(tasks))
	for _, task := range tasks {
		if !task.ReminderAt.Valid {
			continue
		}
		out = append(out, notify.Notification{
			Type:        notify.TypeTaskReminder,
			EntityID:    task.ID,
			TriggerDate: task.ReminderAt.String,
			Message:     notify.TaskReminderMessage(task.Title, formatNotifyDate(task.DueDate.String)),
		})
	}
	return out, nil
}

func (c *Cortex) collectFollowUpNotifications(now time.Time) ([]notify.Notification, error) {
	_ = now
	interactions, err := c.db.FollowUpsDue()
	if err != nil {
		return nil, fmt.Errorf("load follow-up reminders: %w", err)
	}

	out := make([]notify.Notification, 0, len(interactions))
	for _, interaction := range interactions {
		if !interaction.FollowUpDate.Valid {
			continue
		}
		person, err := c.db.GetPerson(interaction.PersonID)
		if err != nil {
			return nil, fmt.Errorf("load follow-up person %s: %w", interaction.PersonID, err)
		}
		if person == nil || person.SuppressedAt.Valid {
			continue
		}
		text := interaction.FollowUp.String
		if strings.TrimSpace(text) == "" {
			text = interaction.Summary
		}
		out = append(out, notify.Notification{
			Type:        notify.TypeFollowUp,
			EntityID:    interaction.ID,
			TriggerDate: interaction.FollowUpDate.String,
			Message: notify.FollowUpMessage(
				person.Name,
				text,
				formatNotifyDate(interaction.Date),
			),
		})
	}
	return out, nil
}

func (c *Cortex) collectBirthdayNotifications(now time.Time) ([]notify.Notification, error) {
	hits, err := c.db.UpcomingBirthdays(3)
	if err != nil {
		return nil, fmt.Errorf("load birthday notifications: %w", err)
	}

	out := make([]notify.Notification, 0, len(hits))
	for _, hit := range hits {
		typ, ok := notify.BirthdayNotificationType(hit.DaysUntil)
		if !ok {
			continue
		}
		triggerDate := now.AddDate(0, 0, hit.DaysUntil).Format("2006-01-02")
		entityID := strings.Join([]string{hit.PersonID, hit.Kind, hit.Name}, ":")
		out = append(out, notify.Notification{
			Type:        typ,
			EntityID:    entityID,
			TriggerDate: triggerDate,
			Message:     notify.BirthdayMessage(hit.Name, hit.Relationship, hit.Kind, hit.DaysUntil),
		})
	}
	return out, nil
}

func (c *Cortex) sendNotificationBatch(ctx context.Context, candidates []notify.Notification) error {
	sendable := make([]notify.Notification, 0, len(candidates))
	for _, candidate := range candidates {
		logEntries, err := c.db.NotificationLog(candidate.Type, candidate.EntityID)
		if err != nil {
			return fmt.Errorf("load notification log %s/%s: %w", candidate.Type, candidate.EntityID, err)
		}
		if notify.ShouldSend(candidate, toNotifyLogEntries(logEntries)) {
			sendable = append(sendable, candidate)
		}
	}
	if len(sendable) == 0 {
		return nil
	}

	message := notify.BatchMessage(sendable)
	if strings.TrimSpace(message) == "" {
		return nil
	}
	if err := c.sendSignal(ctx, message); err != nil {
		return err
	}
	for _, sent := range sendable {
		if err := c.db.LogNotificationTrigger(sent.Type, sent.EntityID, sent.TriggerDate, sent.Message); err != nil {
			return fmt.Errorf("log notification %s/%s: %w", sent.Type, sent.EntityID, err)
		}
	}
	return nil
}

func toNotifyLogEntries(entries []db.NotificationEntry) []notify.LogEntry {
	out := make([]notify.LogEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, notify.LogEntry{
			Type:        entry.RefType,
			EntityID:    entry.RefID,
			TriggerDate: entry.TriggerDate,
		})
	}
	return out
}

func formatNotifyDate(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t.Format("Mon Jan 2")
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Format("Mon Jan 2")
	}
	return value
}
