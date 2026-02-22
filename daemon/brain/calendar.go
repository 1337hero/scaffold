package brain

import (
	"context"

	googlecal "scaffold/google"
)

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
