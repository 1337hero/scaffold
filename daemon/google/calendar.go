package google

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Event struct {
	ID          string
	Title       string
	Start       string
	End         string
	Location    string
	Description string
	AllDay      bool
}

type CalendarClient struct {
	service    *calendar.Service
	CalendarID string
}

func (c *CalendarClient) ensureConfigured() error {
	if c == nil || c.service == nil {
		return fmt.Errorf("calendar client is not configured")
	}
	return nil
}

func normalizeCalendarID(calendarID string) string {
	if strings.TrimSpace(calendarID) == "" {
		return "primary"
	}
	return calendarID
}

func NewCalendarClient(ctx context.Context, tokenSource oauth2.TokenSource, calendarID string) (*CalendarClient, error) {
	srv, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("create calendar service: %w", err)
	}
	if calendarID == "" {
		calendarID = "primary"
	}
	return &CalendarClient{service: srv, CalendarID: calendarID}, nil
}

func (c *CalendarClient) calendarTimezone(ctx context.Context, calendarID string) (*time.Location, error) {
	if err := c.ensureConfigured(); err != nil {
		return nil, err
	}
	calendarID = normalizeCalendarID(calendarID)

	cal, err := c.service.Calendars.Get(calendarID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("get calendar timezone: %w", err)
	}
	loc, err := time.LoadLocation(cal.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("parse timezone %q: %w", cal.TimeZone, err)
	}
	return loc, nil
}

func (c *CalendarClient) TodayEvents(ctx context.Context, calendarID string) ([]Event, error) {
	if err := c.ensureConfigured(); err != nil {
		return nil, err
	}
	calendarID = normalizeCalendarID(calendarID)

	loc, err := c.calendarTimezone(ctx, calendarID)
	if err != nil {
		loc = time.Local
	}

	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.AddDate(0, 0, 1)

	resp, err := c.service.Events.List(calendarID).
		TimeMin(startOfDay.Format(time.RFC3339)).
		TimeMax(endOfDay.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("list today events: %w", err)
	}

	return convertEvents(resp.Items), nil
}

func (c *CalendarClient) UpcomingEvents(ctx context.Context, calendarID string, hours int) ([]Event, error) {
	if err := c.ensureConfigured(); err != nil {
		return nil, err
	}
	calendarID = normalizeCalendarID(calendarID)

	now := time.Now()
	end := now.Add(time.Duration(hours) * time.Hour)

	resp, err := c.service.Events.List(calendarID).
		TimeMin(now.Format(time.RFC3339)).
		TimeMax(end.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("list upcoming events: %w", err)
	}

	return convertEvents(resp.Items), nil
}

func convertEvents(items []*calendar.Event) []Event {
	var events []Event
	for _, item := range items {
		if item.Status == "cancelled" {
			continue
		}
		if isDeclined(item) {
			continue
		}

		e := Event{
			ID:          item.Id,
			Title:       item.Summary,
			Location:    item.Location,
			Description: item.Description,
		}

		if item.Start.Date != "" {
			e.AllDay = true
			e.Start = item.Start.Date
			e.End = item.End.Date
		} else {
			e.Start = item.Start.DateTime
			e.End = item.End.DateTime
		}

		events = append(events, e)
	}
	return events
}

func isDeclined(item *calendar.Event) bool {
	for _, attendee := range item.Attendees {
		if attendee.Self && attendee.ResponseStatus == "declined" {
			return true
		}
	}
	return false
}

func FormatEvents(events []Event) string {
	if len(events) == 0 {
		return "No events found."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d event(s):\n", len(events)))
	for i, e := range events {
		if e.AllDay {
			sb.WriteString(fmt.Sprintf("%d. [All day] %s", i+1, e.Title))
		} else {
			start := formatTime(e.Start)
			end := formatTime(e.End)
			sb.WriteString(fmt.Sprintf("%d. %s–%s %s", i+1, start, end, e.Title))
		}
		if e.Location != "" {
			sb.WriteString(fmt.Sprintf(" @ %s", e.Location))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

func formatTime(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return rfc3339
	}
	return t.Format("3:04 PM")
}
