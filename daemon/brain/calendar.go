package brain

import (
	"context"

	googlecal "scaffold/google"
)

// CalendarToday returns the full day's events (not a lookahead window).
func (b *Brain) CalendarToday(ctx context.Context) ([]googlecal.Event, error) {
	if b == nil || b.calendarClient == nil {
		return nil, nil
	}
	return b.calendarClient.TodayEvents(ctx, b.calendarClient.CalendarID)
}

func (b *Brain) CalendarUpcoming(ctx context.Context, count int) ([]googlecal.Event, error) {
	if b == nil || b.calendarClient == nil {
		return nil, nil
	}
	events, err := b.calendarClient.UpcomingEvents(ctx, b.calendarClient.CalendarID, 8)
	if err != nil {
		return nil, err
	}
	if len(events) > count {
		events = events[:count]
	}
	return events, nil
}
