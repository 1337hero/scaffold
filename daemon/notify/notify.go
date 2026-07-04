package notify

import (
	"bytes"
	"strings"
	"text/template"
)

const (
	TypeTaskReminder  = "reminder"
	TypeFollowUp      = "follow_up"
	TypeBirthdayPoke  = "birthday_poke"
	TypeBirthdayToday = "birthday_today"
)

type Notification struct {
	Type        string
	EntityID    string
	TriggerDate string
	Message     string
}

type LogEntry struct {
	Type        string
	EntityID    string
	TriggerDate string
}

func ShouldSend(notif Notification, log []LogEntry) bool {
	if strings.TrimSpace(notif.Type) == "" ||
		strings.TrimSpace(notif.EntityID) == "" ||
		strings.TrimSpace(notif.TriggerDate) == "" ||
		strings.TrimSpace(notif.Message) == "" {
		return false
	}
	for _, entry := range log {
		if entry.Type == notif.Type &&
			entry.EntityID == notif.EntityID &&
			entry.TriggerDate == notif.TriggerDate {
			return false
		}
	}
	return true
}

func BirthdayNotificationType(daysUntil int) (string, bool) {
	switch daysUntil {
	case 3:
		return TypeBirthdayPoke, true
	case 0:
		return TypeBirthdayToday, true
	default:
		return "", false
	}
}

type taskData struct {
	Title string
	Due   string
}

type followUpData struct {
	Person          string
	Text            string
	LastInteraction string
}

type birthdayData struct {
	Name         string
	Relationship string
	Kind         string
	DaysUntil    int
}

var (
	taskTemplate = template.Must(template.New("task").Parse(
		`Reminder: you set this reminder for {{.Title}}.{{if .Due}} Due {{.Due}}.{{end}}`,
	))
	followUpTemplate = template.Must(template.New("follow-up").Parse(
		`Follow up with {{.Person}}: {{.Text}}.{{if .LastInteraction}} Last interaction: {{.LastInteraction}}.{{end}}`,
	))
	birthdayTemplate = template.Must(template.New("birthday").Parse(
		`{{if .Relationship}}{{.Relationship}} {{end}}{{.Name}}'s {{.Kind}}{{if eq .DaysUntil 3}} is in 3 days. Want to plan something?{{else}} is today.{{end}}`,
	))
)

func TaskReminderMessage(title, due string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "this task"
	}
	return executeTemplate(taskTemplate, taskData{Title: title, Due: strings.TrimSpace(due)})
}

func FollowUpMessage(person, text, lastInteraction string) string {
	person = strings.TrimSpace(person)
	if person == "" {
		person = "this person"
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = "follow up"
	}
	return executeTemplate(followUpTemplate, followUpData{
		Person:          person,
		Text:            text,
		LastInteraction: strings.TrimSpace(lastInteraction),
	})
}

func BirthdayMessage(name, relationship, kind string, daysUntil int) string {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Someone"
	}
	kind = birthdayKindLabel(kind)
	return executeTemplate(birthdayTemplate, birthdayData{
		Name:         name,
		Relationship: strings.TrimSpace(relationship),
		Kind:         kind,
		DaysUntil:    daysUntil,
	})
}

func BatchMessage(notifications []Notification) string {
	if len(notifications) == 0 {
		return ""
	}
	if len(notifications) == 1 {
		return strings.TrimSpace(notifications[0].Message)
	}
	var b strings.Builder
	b.WriteString("Notifications")
	for _, notif := range notifications {
		msg := strings.TrimSpace(notif.Message)
		if msg == "" {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(msg)
	}
	return b.String()
}

func birthdayKindLabel(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "anniversary":
		return "anniversary"
	default:
		return "birthday"
	}
}

func executeTemplate(t *template.Template, data any) string {
	var out bytes.Buffer
	if err := t.Execute(&out, data); err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}
