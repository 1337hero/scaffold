package brief

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
	"time"
)

const (
	SurfaceBusiness = "business"
	SurfaceLife     = "life"
)

type QueryData struct {
	GeneratedAt      time.Time
	CalendarEvents   []Event
	TasksDue         []Task
	SlippingProjects []SlippingItem
	SlippingAreas    []SlippingItem
	Reminders        []Task
	FollowUps        []FollowUp
	Birthdays        []Birthday
	OverduePeople    []Person
}

type Event struct {
	Title    string
	Start    string
	End      string
	Location string
	AllDay   bool
}

type Task struct {
	Title      string
	DueDate    string
	ReminderAt string
	Priority   string
}

type SlippingItem struct {
	Name      string
	DaysStale int
}

type FollowUp struct {
	Title   string
	Summary string
	Date    string
	Person  string
}

type Birthday struct {
	Name         string
	Kind         string
	Date         string
	DaysUntil    int
	Relationship string
}

type Person struct {
	Name              string
	LastInteractionAt string
	DaysSinceContact  int
}

type section struct {
	Title string
	Lines []string
}

type viewModel struct {
	Title       string
	GeneratedAt string
	Sections    []section
	Empty       bool
}

const briefTemplate = `{{.Title}}{{if .GeneratedAt}}
{{.GeneratedAt}}{{end}}{{if .Empty}}

Nothing pressing.{{else}}{{range .Sections}}

{{.Title}}{{range .Lines}}
- {{.}}{{end}}{{end}}{{end}}`

var tmpl = template.Must(template.New("daily-brief").Parse(briefTemplate))

func AssembleBrief(surface string, data QueryData) string {
	surface = normalizeSurface(surface)
	sections := sectionsForSurface(surface, data)
	model := viewModel{
		Title:    briefTitle(surface),
		Sections: sections,
		Empty:    len(sections) == 0,
	}
	if !data.GeneratedAt.IsZero() {
		model.GeneratedAt = data.GeneratedAt.Format("Mon Jan 2, 3:04 PM MST")
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, model); err != nil {
		return model.Title
	}
	return strings.TrimSpace(out.String())
}

func normalizeSurface(surface string) string {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case SurfaceBusiness:
		return SurfaceBusiness
	default:
		return SurfaceLife
	}
}

func briefTitle(surface string) string {
	if surface == SurfaceBusiness {
		return "BusinessOS morning brief"
	}
	return "LifeOS evening brief"
}

func sectionsForSurface(surface string, data QueryData) []section {
	if surface == SurfaceBusiness {
		return compactSections([]section{
			{Title: "Calendar today", Lines: eventLines(data.CalendarEvents)},
			{Title: "Due tasks", Lines: taskDueLines(data.TasksDue)},
			{Title: "Slipping work", Lines: append(slippingLines(data.SlippingProjects), slippingLines(data.SlippingAreas)...)},
			{Title: "Morning reminders", Lines: reminderLines(data.Reminders)},
			{Title: "Follow-ups due", Lines: followUpLines(data.FollowUps)},
		})
	}

	return compactSections([]section{
		{Title: "Calendar tomorrow", Lines: eventLines(data.CalendarEvents)},
		{Title: "Upcoming dates", Lines: birthdayLines(data.Birthdays)},
		{Title: "Slipping life areas", Lines: slippingLines(data.SlippingAreas)},
		{Title: "Life reminders", Lines: reminderLines(data.Reminders)},
		{Title: "Follow-ups due", Lines: followUpLines(data.FollowUps)},
		{Title: "Overdue interactions", Lines: personLines(data.OverduePeople)},
	})
}

func compactSections(in []section) []section {
	out := make([]section, 0, len(in))
	for _, s := range in {
		if len(s.Lines) > 0 {
			out = append(out, s)
		}
	}
	return out
}

func eventLines(events []Event) []string {
	out := make([]string, 0, len(events))
	for _, event := range events {
		title := strings.TrimSpace(event.Title)
		if title == "" {
			continue
		}
		prefix := "All day"
		if !event.AllDay {
			prefix = formatTimeRange(event.Start, event.End)
		}
		line := fmt.Sprintf("%s: %s", prefix, title)
		if loc := strings.TrimSpace(event.Location); loc != "" {
			line += " @ " + loc
		}
		out = append(out, line)
	}
	return out
}

func taskDueLines(tasks []Task) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		title := strings.TrimSpace(task.Title)
		if title == "" {
			continue
		}
		if due := strings.TrimSpace(task.DueDate); due != "" {
			out = append(out, fmt.Sprintf("%s (due %s)", title, formatDate(due)))
		} else {
			out = append(out, title)
		}
	}
	return out
}

func reminderLines(tasks []Task) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		title := strings.TrimSpace(task.Title)
		if title == "" {
			continue
		}
		if at := strings.TrimSpace(task.ReminderAt); at != "" {
			out = append(out, fmt.Sprintf("%s (%s)", title, formatTime(at)))
		} else {
			out = append(out, title)
		}
	}
	return out
}

func slippingLines(items []SlippingItem) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if item.DaysStale > 0 {
			out = append(out, fmt.Sprintf("%s (%d days)", name, item.DaysStale))
		} else {
			out = append(out, name+" (no activity yet)")
		}
	}
	return out
}

func followUpLines(items []FollowUp) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		title := strings.TrimSpace(item.Title)
		if title == "" {
			title = strings.TrimSpace(item.Summary)
		}
		if title == "" {
			continue
		}
		line := title
		if person := strings.TrimSpace(item.Person); person != "" {
			line += " - " + person
		}
		if date := strings.TrimSpace(item.Date); date != "" {
			line += " (" + formatDate(date) + ")"
		}
		if summary := strings.TrimSpace(item.Summary); summary != "" && summary != title {
			line += ": " + summary
		}
		out = append(out, line)
	}
	return out
}

func birthdayLines(items []Birthday) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		parts := []string{}
		if relationship := strings.TrimSpace(item.Relationship); relationship != "" {
			parts = append(parts, relationship)
		}
		if kind := strings.TrimSpace(item.Kind); kind != "" {
			parts = append(parts, kind)
		}
		label := name
		if len(parts) > 0 {
			label += " (" + strings.Join(parts, ", ") + ")"
		}
		out = append(out, fmt.Sprintf("%s - %s", label, formatDaysUntil(item.DaysUntil)))
	}
	return out
}

func personLines(items []Person) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		line := name
		if item.DaysSinceContact > 0 {
			line += fmt.Sprintf(" (%d days)", item.DaysSinceContact)
		}
		if last := strings.TrimSpace(item.LastInteractionAt); last != "" {
			line += " - last contact " + formatDate(last)
		}
		out = append(out, line)
	}
	return out
}

func formatDaysUntil(days int) string {
	switch days {
	case 0:
		return "today"
	case 1:
		return "tomorrow"
	default:
		return fmt.Sprintf("in %d days", days)
	}
}

func formatTimeRange(start, end string) string {
	start = formatTime(start)
	end = formatTime(end)
	if start == "" {
		return "Time unknown"
	}
	if end == "" || end == start {
		return start
	}
	return start + "-" + end
}

func formatTime(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Format("3:04 PM")
	}
	if t, err := time.Parse("15:04", value); err == nil {
		return t.Format("3:04 PM")
	}
	return value
}

func formatDate(value string) string {
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
