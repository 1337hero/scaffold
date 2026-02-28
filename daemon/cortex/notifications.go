package cortex

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"scaffold/db"
	"scaffold/sessionbus"
)

type NotificationMessage struct {
	Type    string `json:"type"`
	SubType string `json:"sub_type"`
	Text    string `json:"text"`
}

func (c *Cortex) runNotifications(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("database is nil")
	}
	if c.notifyCfg == nil || !c.notifyCfg.Enabled {
		return nil
	}
	if c.sessionBus == nil {
		return fmt.Errorf("session bus is nil")
	}

	now := time.Now()
	todayStr := now.Format("2006-01-02")
	sent := 0

	tasks, err := c.db.NotifiableTasks(todayStr)
	if err != nil {
		return fmt.Errorf("query notifiable tasks: %w", err)
	}

	morningHit := c.inWindow(now, c.notifyCfg.Reminders.MorningWindow)
	checkinHit := c.inWindow(now, c.notifyCfg.Reminders.CheckinWindow)
	overdueCooldown := time.Duration(c.notifyCfg.OverdueCooldownHours) * time.Hour

	for _, t := range tasks {
		dueToday := t.DueDate.Valid && t.DueDate.String <= todayStr
		overdue := t.DueDate.Valid && t.DueDate.String < todayStr
		isRecurring := t.Recurring.Valid

		if overdue {
			sent += c.sendOverdueNudge(ctx, t, overdueCooldown)
		} else if dueToday && morningHit {
			sent += c.sendDueReminder(ctx, t)
		} else if isRecurring && checkinHit && c.isRecurringDay(t, now) {
			sent += c.sendRecurringCheckin(ctx, t)
		}
	}

	if c.notifyCfg.Briefing.Enabled {
		sent += c.sendDailyBriefing(ctx, now, todayStr, tasks)
	}

	log.Printf("cortex: notifications sent=%d", sent)
	return nil
}

func (c *Cortex) sendDueReminder(ctx context.Context, t db.Task) int {
	alreadySent, err := c.db.NotificationSentSince("task_due", t.ID, startOfDay(time.Now()))
	if err != nil || alreadySent {
		return 0
	}

	msg := fmt.Sprintf("📋 %s is due today.", t.Title)
	if t.DomainName != "" {
		msg += fmt.Sprintf(" [%s]", t.DomainName)
	}

	c.sendNotification(ctx, "task_due", t.ID, msg)
	return 1
}

func (c *Cortex) sendOverdueNudge(ctx context.Context, t db.Task, cooldown time.Duration) int {
	since := time.Now().Add(-cooldown)
	alreadySent, err := c.db.NotificationSentSince("task_overdue", t.ID, since)
	if err != nil || alreadySent {
		return 0
	}

	daysOverdue := daysBetween(t.DueDate.String, time.Now().Format("2006-01-02"))
	var msg string
	if daysOverdue == 1 {
		msg = fmt.Sprintf("⏰ %s was due yesterday and isn't done yet.", t.Title)
	} else {
		msg = fmt.Sprintf("⏰ %s is %d days overdue.", t.Title, daysOverdue)
	}

	c.sendNotification(ctx, "task_overdue", t.ID, msg)
	return 1
}

func (c *Cortex) sendRecurringCheckin(ctx context.Context, t db.Task) int {
	alreadySent, err := c.db.NotificationSentSince("task_checkin", t.ID, startOfDay(time.Now()))
	if err != nil || alreadySent {
		return 0
	}

	msg := fmt.Sprintf("🔄 Your recurring task \"%s\" is scheduled for today — done yet?", t.Title)
	c.sendNotification(ctx, "task_checkin", t.ID, msg)
	return 1
}

func (c *Cortex) sendDailyBriefing(ctx context.Context, now time.Time, todayStr string, tasks []db.Task) int {
	if !c.inWindow(now, c.notifyCfg.Briefing.Schedule) {
		return 0
	}
	if !c.isBriefingDay(now) {
		return 0
	}

	alreadySent, err := c.db.NotificationSentSince("briefing", "daily", startOfDay(now))
	if err != nil || alreadySent {
		return 0
	}

	var dueToday, overdue, recurring int
	for _, t := range tasks {
		if t.DueDate.Valid && t.DueDate.String < todayStr {
			overdue++
		} else if t.DueDate.Valid && t.DueDate.String == todayStr {
			dueToday++
		}
		if t.Recurring.Valid && c.isRecurringDay(t, now) {
			recurring++
		}
	}

	var sb strings.Builder
	sb.WriteString("☀️ Daily Briefing\n")

	if dueToday > 0 {
		sb.WriteString(fmt.Sprintf("• %d task(s) due today\n", dueToday))
	}
	if overdue > 0 {
		sb.WriteString(fmt.Sprintf("• %d overdue task(s)\n", overdue))
	}
	if recurring > 0 {
		sb.WriteString(fmt.Sprintf("• %d recurring task(s) scheduled\n", recurring))
	}
	if dueToday == 0 && overdue == 0 && recurring == 0 {
		sb.WriteString("• No notifiable tasks today. 🎉\n")
	}

	if bulletin, fresh := c.bulletin.Get(); bulletin != "" && fresh {
		if len(bulletin) > 300 {
			bulletin = bulletin[:300] + "..."
		}
		sb.WriteString(fmt.Sprintf("\n📝 Context:\n%s", bulletin))
	}

	c.sendNotification(ctx, "briefing", "daily", sb.String())
	return 1
}

func (c *Cortex) sendNotification(ctx context.Context, subType, refID, text string) {
	payload, _ := json.Marshal(NotificationMessage{
		Type:    "notification",
		SubType: subType,
		Text:    text,
	})

	_, err := c.sessionBus.Send(ctx, sessionbus.SendRequest{
		FromSessionID: "cortex",
		ToSessionID:   "scaffold-agent",
		Message:       string(payload),
	})
	if err != nil {
		log.Printf("cortex: notification send failed: %v", err)
		return
	}

	if err := c.db.LogNotification(subType, refID, text); err != nil {
		log.Printf("cortex: notification log failed: %v", err)
	}
}

func (c *Cortex) inWindow(now time.Time, window string) bool {
	parts := strings.SplitN(window, ":", 2)
	if len(parts) != 2 {
		return false
	}
	var h, m int
	fmt.Sscanf(parts[0], "%d", &h)
	fmt.Sscanf(parts[1], "%d", &m)

	windowTime := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
	diff := now.Sub(windowTime)
	return diff >= 0 && diff < 15*time.Minute
}

func (c *Cortex) isBriefingDay(now time.Time) bool {
	dayName := strings.ToLower(now.Weekday().String()[:3])
	for _, d := range c.notifyCfg.Briefing.Days {
		if strings.ToLower(d) == dayName {
			return true
		}
	}
	return false
}

func (c *Cortex) isRecurringDay(t db.Task, now time.Time) bool {
	if !t.Recurring.Valid {
		return false
	}
	switch t.Recurring.String {
	case "daily":
		return true
	case "weekly":
		return t.DueDate.Valid && sameWeekday(t.DueDate.String, now)
	case "monthly":
		return t.DueDate.Valid && sameDayOfMonth(t.DueDate.String, now)
	default:
		return false
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func daysBetween(dateStr string, todayStr string) int {
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 1
	}
	t, err := time.Parse("2006-01-02", todayStr)
	if err != nil {
		return 1
	}
	days := int(t.Sub(d).Hours() / 24)
	if days < 1 {
		return 1
	}
	return days
}

func sameWeekday(dateStr string, now time.Time) bool {
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}
	return d.Weekday() == now.Weekday()
}

func sameDayOfMonth(dateStr string, now time.Time) bool {
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return false
	}
	return d.Day() == now.Day()
}
