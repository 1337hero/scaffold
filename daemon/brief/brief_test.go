package brief

import (
	"strings"
	"testing"
	"time"
)

func TestAssembleBriefEmptyBusiness(t *testing.T) {
	got := AssembleBrief(SurfaceBusiness, QueryData{})

	if !strings.Contains(got, "BusinessOS morning brief") {
		t.Fatalf("missing title: %s", got)
	}
	if !strings.Contains(got, "Nothing pressing.") {
		t.Fatalf("missing empty state: %s", got)
	}
	if strings.Contains(got, "Calendar today") {
		t.Fatalf("empty section should be omitted: %s", got)
	}
}

func TestAssembleBriefBusinessSections(t *testing.T) {
	generated := time.Date(2026, time.July, 4, 9, 0, 0, 0, time.FixedZone("MST", -7*60*60))
	got := AssembleBrief(SurfaceBusiness, QueryData{
		GeneratedAt: generated,
		CalendarEvents: []Event{{
			Title: "Client standup",
			Start: "2026-07-04T09:30:00-06:00",
			End:   "2026-07-04T10:00:00-06:00",
		}},
		TasksDue: []Task{{
			Title:   "Ship PRD 16",
			DueDate: "2026-07-04",
		}},
		SlippingProjects: []SlippingItem{{Name: "Website redesign", DaysStale: 8}},
		Reminders:        []Task{{Title: "Send invoice", ReminderAt: "2026-07-04T11:00:00-06:00"}},
		FollowUps:        []FollowUp{{Title: "Send proposal", Summary: "Kickoff call", Person: "Taylor", Date: "2026-07-04"}},
	})

	required := []string{
		"BusinessOS morning brief",
		"Sat Jul 4, 9:00 AM MST",
		"Calendar today",
		"9:30 AM-10:00 AM: Client standup",
		"Due tasks",
		"Ship PRD 16 (due Sat Jul 4)",
		"Slipping work",
		"Website redesign (8 days)",
		"Morning reminders",
		"Send invoice (11:00 AM)",
		"Follow-ups due",
		"Send proposal - Taylor (Sat Jul 4): Kickoff call",
	}
	for _, want := range required {
		if !strings.Contains(got, want) {
			t.Fatalf("brief missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Upcoming dates") {
		t.Fatalf("business brief included life section:\n%s", got)
	}
}

func TestAssembleBriefLifeSections(t *testing.T) {
	got := AssembleBrief(SurfaceLife, QueryData{
		CalendarEvents: []Event{{Title: "Family dinner", AllDay: true}},
		Birthdays: []Birthday{
			{Name: "Auggie", Kind: "self", Relationship: "family", DaysUntil: 0},
			{Name: "Marcus", Kind: "kid", Relationship: "family", DaysUntil: 3},
		},
		SlippingAreas: []SlippingItem{{Name: "Garden", DaysStale: 14}},
		Reminders:     []Task{{Title: "Buy gift", ReminderAt: "2026-07-04T18:30:00-06:00"}},
		FollowUps:     []FollowUp{{Summary: "Text mom", Person: "Mom"}},
		OverduePeople: []Person{{Name: "Sam", LastInteractionAt: "2026-04-01", DaysSinceContact: 94}},
	})

	required := []string{
		"LifeOS evening brief",
		"Calendar tomorrow",
		"All day: Family dinner",
		"Upcoming dates",
		"Auggie (family, self) - today",
		"Marcus (family, kid) - in 3 days",
		"Slipping life areas",
		"Garden (14 days)",
		"Life reminders",
		"Buy gift (6:30 PM)",
		"Follow-ups due",
		"Text mom - Mom",
		"Overdue interactions",
		"Sam (94 days) - last contact Wed Apr 1",
	}
	for _, want := range required {
		if !strings.Contains(got, want) {
			t.Fatalf("brief missing %q:\n%s", want, got)
		}
	}
}

func TestFormatDaysUntil(t *testing.T) {
	cases := map[int]string{
		0: "today",
		1: "tomorrow",
		7: "in 7 days",
	}
	for days, want := range cases {
		if got := formatDaysUntil(days); got != want {
			t.Fatalf("formatDaysUntil(%d)=%q, want %q", days, got, want)
		}
	}
}
