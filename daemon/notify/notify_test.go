package notify

import (
	"strings"
	"testing"
)

func TestShouldSend(t *testing.T) {
	notif := Notification{Type: TypeTaskReminder, EntityID: "task1", TriggerDate: "2026-07-04T09:00:00Z", Message: "go"}

	cases := []struct {
		name string
		log  []LogEntry
		want bool
	}{
		{name: "first send", want: true},
		{name: "duplicate", log: []LogEntry{{Type: TypeTaskReminder, EntityID: "task1", TriggerDate: "2026-07-04T09:00:00Z"}}, want: false},
		{name: "different date", log: []LogEntry{{Type: TypeTaskReminder, EntityID: "task1", TriggerDate: "2026-07-05T09:00:00Z"}}, want: true},
		{name: "different entity", log: []LogEntry{{Type: TypeTaskReminder, EntityID: "task2", TriggerDate: "2026-07-04T09:00:00Z"}}, want: true},
		{name: "different type", log: []LogEntry{{Type: TypeFollowUp, EntityID: "task1", TriggerDate: "2026-07-04T09:00:00Z"}}, want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ShouldSend(notif, tc.log); got != tc.want {
				t.Fatalf("ShouldSend=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestShouldSendRejectsIncompleteNotification(t *testing.T) {
	if ShouldSend(Notification{Type: TypeTaskReminder, EntityID: "task1", TriggerDate: "today"}, nil) {
		t.Fatal("missing message should not send")
	}
}

func TestBirthdayNotificationType(t *testing.T) {
	cases := map[int]struct {
		typ string
		ok  bool
	}{
		7:  {"", false},
		3:  {TypeBirthdayPoke, true},
		0:  {TypeBirthdayToday, true},
		-1: {"", false},
	}
	for days, want := range cases {
		got, ok := BirthdayNotificationType(days)
		if got != want.typ || ok != want.ok {
			t.Fatalf("BirthdayNotificationType(%d)=(%q,%v), want (%q,%v)", days, got, ok, want.typ, want.ok)
		}
	}
}

func TestMessageTemplates(t *testing.T) {
	task := TaskReminderMessage("Pay tax estimate", "Mon Jul 6")
	if !strings.Contains(task, "you set this reminder") || !strings.Contains(task, "Due Mon Jul 6") {
		t.Fatalf("bad task message: %s", task)
	}

	follow := FollowUpMessage("Taylor", "send proposal", "Fri Jul 3")
	if !strings.Contains(follow, "Follow up with Taylor: send proposal") || !strings.Contains(follow, "Last interaction: Fri Jul 3") {
		t.Fatalf("bad follow-up message: %s", follow)
	}

	poke := BirthdayMessage("Auggie", "family", "self", 3)
	if !strings.Contains(poke, "family Auggie's birthday is in 3 days") || !strings.Contains(poke, "Want to plan something?") {
		t.Fatalf("bad birthday poke: %s", poke)
	}

	today := BirthdayMessage("Launch", "", "anniversary", 0)
	if !strings.Contains(today, "Launch's anniversary is today") {
		t.Fatalf("bad birthday today message: %s", today)
	}
}

func TestBatchMessage(t *testing.T) {
	msg := BatchMessage([]Notification{
		{Message: "one"},
		{Message: "two"},
	})
	if !strings.Contains(msg, "Notifications") || !strings.Contains(msg, "- one") || !strings.Contains(msg, "- two") {
		t.Fatalf("bad batch: %s", msg)
	}
}
