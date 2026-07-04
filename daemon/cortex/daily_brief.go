package cortex

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"scaffold/brief"
	appconfig "scaffold/config"
	"scaffold/db"
	googlecal "scaffold/google"
)

const dailyBriefTimeout = 30 * time.Second

func (c *Cortex) SetNotificationsConfig(cfg appconfig.NotificationsConfig) {
	if c == nil {
		return
	}
	c.notifyCfg = cfg
}

func (c *Cortex) SetBriefSender(sender func(ctx context.Context, message string) error) {
	c.SetSignalSender(sender)
}

func (c *Cortex) SetSignalSender(sender func(ctx context.Context, message string) error) {
	if c == nil {
		return
	}
	c.sendSignal = sender
}

func (c *Cortex) maybeRunDailyBriefs(ctx context.Context, now time.Time) {
	if c == nil || !c.dailyBriefsEnabled() {
		return
	}
	loc := briefLocation()
	localNow := now.In(loc)

	cfg := c.notifyCfg.Briefing
	if sameBriefMinute(localNow, cfg.MorningSchedule) {
		c.startDailyBrief(ctx, "morning_brief", brief.SurfaceBusiness, localNow)
	}
	if sameBriefMinute(localNow, cfg.EveningSchedule) {
		c.startDailyBrief(ctx, "evening_brief", brief.SurfaceLife, localNow)
	}
}

func (c *Cortex) dailyBriefsEnabled() bool {
	return c.notifyCfg.Enabled && c.notifyCfg.Briefing.Enabled
}

func sameBriefMinute(now time.Time, schedule string) bool {
	schedule = strings.TrimSpace(schedule)
	if schedule == "" {
		return false
	}
	parsed, err := time.Parse("15:04", schedule)
	if err != nil {
		return false
	}
	return now.Hour() == parsed.Hour() && now.Minute() == parsed.Minute()
}

func (c *Cortex) startDailyBrief(ctx context.Context, name, surface string, localNow time.Time) {
	if !c.claimBriefRun(name, localNow) {
		return
	}

	go func() {
		taskCtx, cancel := context.WithTimeout(ctx, dailyBriefTimeout)
		defer cancel()

		start := time.Now()
		if err := c.sendDailyBrief(taskCtx, surface); err != nil {
			log.Printf("cortex: %s failed (%v): %v", name, time.Since(start), err)
			return
		}
		log.Printf("cortex: %s sent (%v)", name, time.Since(start))
	}()
}

func (c *Cortex) claimBriefRun(name string, localNow time.Time) bool {
	c.briefMu.Lock()
	defer c.briefMu.Unlock()

	if c.briefRuns == nil {
		c.briefRuns = make(map[string]struct{})
	}
	key := name + ":" + localNow.Format("2006-01-02")
	if _, ok := c.briefRuns[key]; ok {
		return false
	}
	c.briefRuns[key] = struct{}{}
	return true
}

func (c *Cortex) sendDailyBrief(ctx context.Context, surface string) error {
	if c.sendSignal == nil {
		return fmt.Errorf("brief sender is not configured")
	}
	data, err := c.loadBriefData(ctx, surface)
	if err != nil {
		return err
	}
	message := brief.AssembleBrief(surface, data)
	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("brief assembler returned blank message")
	}
	return c.sendSignal(ctx, message)
}

func (c *Cortex) loadBriefData(ctx context.Context, surface string) (brief.QueryData, error) {
	if c.db == nil {
		return brief.QueryData{}, fmt.Errorf("database is nil")
	}

	loc := briefLocation()
	now := time.Now().In(loc)
	data := brief.QueryData{GeneratedAt: now}

	surface = strings.ToLower(strings.TrimSpace(surface))
	switch surface {
	case brief.SurfaceBusiness:
		events, err := c.calendarToday(ctx)
		if err != nil {
			log.Printf("cortex: morning brief calendar unavailable: %v", err)
		}
		data.CalendarEvents = toBriefEvents(events)

		tasks, err := c.db.ListTasks(db.TaskFilters{Surface: &surface, Status: "pending", Due: "today"})
		if err != nil {
			return brief.QueryData{}, fmt.Errorf("load due business tasks: %w", err)
		}
		data.TasksDue = toBriefTasks(tasks)

		projects, err := c.db.ProjectsSlipping(&surface)
		if err != nil {
			return brief.QueryData{}, fmt.Errorf("load slipping business projects: %w", err)
		}
		data.SlippingProjects = toBriefSlipping(projects, now)

		areas, err := c.db.AreasSlipping(&surface)
		if err != nil {
			return brief.QueryData{}, fmt.Errorf("load slipping business areas: %w", err)
		}
		data.SlippingAreas = toBriefSlipping(areas, now)

		noon := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, loc).UTC().Format(time.RFC3339)
		reminders, err := c.db.ListTasks(db.TaskFilters{Surface: &surface, Status: "pending", ReminderAt: &noon})
		if err != nil {
			return brief.QueryData{}, fmt.Errorf("load business reminders: %w", err)
		}
		data.Reminders = toBriefTasks(reminders)

		followUps, err := c.followUpsDue(surface)
		if err != nil {
			return brief.QueryData{}, err
		}
		data.FollowUps = followUps
	default:
		surface = brief.SurfaceLife
		events, err := c.calendarTomorrow(ctx)
		if err != nil {
			log.Printf("cortex: evening brief calendar unavailable: %v", err)
		}
		data.CalendarEvents = toBriefEvents(events)

		birthdays, err := c.db.UpcomingBirthdays(7)
		if err != nil {
			return brief.QueryData{}, fmt.Errorf("load birthdays: %w", err)
		}
		data.Birthdays = toBriefBirthdays(birthdays)

		areas, err := c.db.AreasSlipping(&surface)
		if err != nil {
			return brief.QueryData{}, fmt.Errorf("load slipping life areas: %w", err)
		}
		data.SlippingAreas = toBriefSlipping(areas, now)

		reminderCutoff := now.UTC().Format(time.RFC3339)
		reminders, err := c.db.ListTasks(db.TaskFilters{Surface: &surface, Status: "pending", ReminderAt: &reminderCutoff})
		if err != nil {
			return brief.QueryData{}, fmt.Errorf("load life reminders: %w", err)
		}
		data.Reminders = toBriefTasks(reminders)

		followUps, err := c.followUpsDue(surface)
		if err != nil {
			return brief.QueryData{}, err
		}
		data.FollowUps = followUps

		people, err := c.db.PeopleSlipping(&surface)
		if err != nil {
			return brief.QueryData{}, fmt.Errorf("load overdue people: %w", err)
		}
		data.OverduePeople = toBriefPeople(people, now)
	}

	return data, nil
}

func (c *Cortex) calendarToday(ctx context.Context) ([]googlecal.Event, error) {
	if c.brain == nil {
		return nil, nil
	}
	return c.brain.CalendarToday(ctx)
}

func (c *Cortex) calendarTomorrow(ctx context.Context) ([]googlecal.Event, error) {
	if c.brain == nil {
		return nil, nil
	}
	return c.brain.CalendarTomorrow(ctx)
}

func (c *Cortex) followUpsDue(surface string) ([]brief.FollowUp, error) {
	interactions, err := c.db.FollowUpsDue()
	if err != nil {
		return nil, fmt.Errorf("load follow-ups due: %w", err)
	}

	out := make([]brief.FollowUp, 0, len(interactions))
	for _, interaction := range interactions {
		person, err := c.db.GetPerson(interaction.PersonID)
		if err != nil {
			return nil, fmt.Errorf("load follow-up person %s: %w", interaction.PersonID, err)
		}
		if person == nil || person.SuppressedAt.Valid {
			continue
		}
		if strings.TrimSpace(surface) != "" && person.Surface != surface {
			continue
		}
		title := interaction.FollowUp.String
		if strings.TrimSpace(title) == "" {
			title = interaction.Summary
		}
		out = append(out, brief.FollowUp{
			Title:   title,
			Summary: interaction.Summary,
			Date:    interaction.FollowUpDate.String,
			Person:  person.Name,
		})
	}
	return out, nil
}

func toBriefEvents(events []googlecal.Event) []brief.Event {
	out := make([]brief.Event, 0, len(events))
	for _, event := range events {
		out = append(out, brief.Event{
			Title:    event.Title,
			Start:    event.Start,
			End:      event.End,
			Location: event.Location,
			AllDay:   event.AllDay,
		})
	}
	return out
}

func toBriefTasks(tasks []db.Task) []brief.Task {
	out := make([]brief.Task, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, brief.Task{
			Title:      task.Title,
			DueDate:    task.DueDate.String,
			ReminderAt: task.ReminderAt.String,
			Priority:   task.Priority,
		})
	}
	return out
}

func toBriefSlipping(projects []db.Project, now time.Time) []brief.SlippingItem {
	out := make([]brief.SlippingItem, 0, len(projects))
	for _, project := range projects {
		out = append(out, brief.SlippingItem{
			Name:      project.Name,
			DaysStale: daysSince(firstValid(project.LastActivityAt, project.CreatedAt), now),
		})
	}
	return out
}

func toBriefBirthdays(hits []db.BirthdayHit) []brief.Birthday {
	out := make([]brief.Birthday, 0, len(hits))
	for _, hit := range hits {
		out = append(out, brief.Birthday{
			Name:         hit.Name,
			Kind:         hit.Kind,
			Date:         hit.Date,
			DaysUntil:    hit.DaysUntil,
			Relationship: hit.Relationship,
		})
	}
	return out
}

func toBriefPeople(people []db.Person, now time.Time) []brief.Person {
	out := make([]brief.Person, 0, len(people))
	for _, person := range people {
		out = append(out, brief.Person{
			Name:              person.Name,
			LastInteractionAt: person.LastInteractionAt.String,
			DaysSinceContact:  daysSince(person.LastInteractionAt.String, now),
		})
	}
	return out
}

func firstValid(value sql.NullString, fallback string) string {
	if value.Valid && strings.TrimSpace(value.String) != "" {
		return value.String
	}
	return fallback
}

func daysSince(value string, now time.Time) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var t time.Time
	var err error
	t, err = time.Parse(time.RFC3339, value)
	if err != nil {
		t, err = time.Parse("2006-01-02", value)
	}
	if err != nil {
		return 0
	}
	if t.After(now) {
		return 0
	}
	return int(now.Sub(t).Hours() / 24)
}

func briefLocation() *time.Location {
	loc, err := time.LoadLocation("America/Denver")
	if err != nil {
		return time.FixedZone("MST", -7*60*60)
	}
	return loc
}
