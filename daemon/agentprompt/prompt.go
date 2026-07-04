package agentprompt

import (
	"fmt"
	"strings"
	"time"
)

const (
	SurfaceLife     = "life"
	SurfaceBusiness = "business"
)

type Identity struct {
	Name     string
	UserName string
	Voice    []string
	Values   []string
	Posture  []string
	CannotDo []string
	Rules    []string
}

type Fact struct {
	Entity   string
	Content  string
	Category string
	Trust    float64
}

type CalendarEvent struct {
	Title       string
	Description string
}

func AssembleSystemPrompt(identity Identity, bulletin, surface string, facts []Fact) string {
	identity = normalizeIdentity(identity)
	bulletin = strings.TrimSpace(bulletin)
	if bulletin == "" {
		bulletin = "No bulletin available yet."
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You are %s, %s's personal agent.\n", identity.Name, identity.UserName)
	b.WriteString("You are a witness and external executive-function surface, not an optimizer.\n")

	writeList(&b, "Voice", identity.Voice)
	writeList(&b, "Values", identity.Values)
	writeList(&b, "Posture", identity.Posture)

	b.WriteString("\n## Current Surface\n")
	switch NormalizeSurface(surface) {
	case SurfaceBusiness:
		b.WriteString("BusinessOS. Frame work, clients, projects, and delivery clearly.\n")
	default:
		b.WriteString("LifeOS. Frame home, family, health, relationships, and personal context clearly.\n")
	}

	b.WriteString("\n## Current Context\n")
	b.WriteString(bulletin)
	b.WriteString("\n")

	if len(facts) > 0 {
		b.WriteString("\n## High-Trust Facts\n")
		for _, fact := range facts {
			content := strings.TrimSpace(fact.Content)
			if content == "" {
				continue
			}
			entity := strings.TrimSpace(fact.Entity)
			if entity == "" {
				entity = "Context"
			}
			fmt.Fprintf(&b, "- %s: %s", entity, content)
			if fact.Trust > 0 {
				fmt.Fprintf(&b, " (trust %.2f)", fact.Trust)
			}
			if category := strings.TrimSpace(fact.Category); category != "" {
				fmt.Fprintf(&b, " [%s]", category)
			}
			b.WriteString("\n")
		}
	}

	writeList(&b, "Cannot Do", identity.CannotDo)
	writeList(&b, "Operating Rules", identity.Rules)

	return strings.TrimSpace(b.String())
}

func DetectSurface(explicitSwitch string, at time.Time, events []CalendarEvent, queryContent string) string {
	if surface, ok := inferSurfaceFromText(explicitSwitch, true); ok {
		return surface
	}
	if surface, ok := inferSurfaceFromText(queryContent, false); ok {
		return surface
	}
	if surface, ok := inferSurfaceFromCalendar(events); ok {
		return surface
	}
	if isBusinessTime(at) {
		return SurfaceBusiness
	}
	return SurfaceLife
}

func NormalizeSurface(surface string) string {
	switch strings.ToLower(strings.TrimSpace(surface)) {
	case "business", "businessos", "work", "workos":
		return SurfaceBusiness
	default:
		return SurfaceLife
	}
}

func normalizeIdentity(identity Identity) Identity {
	identity.Name = strings.TrimSpace(identity.Name)
	if identity.Name == "" {
		identity.Name = "Scaffold"
	}
	identity.UserName = strings.TrimSpace(identity.UserName)
	if identity.UserName == "" {
		identity.UserName = "Mike"
	}
	if len(nonBlank(identity.Voice)) == 0 {
		identity.Voice = []string{"Direct, warm, concise."}
	}
	if len(nonBlank(identity.Values)) == 0 {
		identity.Values = []string{"Surface what is true without deciding what matters."}
	}
	if len(nonBlank(identity.Posture)) == 0 {
		identity.Posture = []string{"Witness, remind, and connect context across time."}
	}
	if len(nonBlank(identity.CannotDo)) == 0 {
		identity.CannotDo = []string{
			"Decide what is important for Mike.",
			"Set the Top 3.",
			"Create tasks unless Mike asks.",
			"Access email.",
			"Run code.",
		}
	}
	identity.Voice = nonBlank(identity.Voice)
	identity.Values = nonBlank(identity.Values)
	identity.Posture = nonBlank(identity.Posture)
	identity.CannotDo = nonBlank(identity.CannotDo)
	identity.Rules = nonBlank(identity.Rules)
	return identity
}

func nonBlank(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func writeList(b *strings.Builder, title string, values []string) {
	values = nonBlank(values)
	if len(values) == 0 {
		return
	}
	b.WriteString("\n## ")
	b.WriteString(title)
	b.WriteString("\n")
	for _, value := range values {
		b.WriteString("- ")
		b.WriteString(value)
		b.WriteString("\n")
	}
}

func inferSurfaceFromText(text string, explicit bool) (string, bool) {
	text = normalizeSurfaceText(text)
	if strings.TrimSpace(text) == "" {
		return "", false
	}

	lifeTerms := []string{
		" life mode ", " lifeos ", " personal mode ", " home ", " house ", " fence ",
		" family ", " kid ", " kids ", " birthday ", " church ", " health ", " dog ",
		" errand ", " errands ", " yard ", " relationship ",
	}
	businessTerms := []string{
		" business mode ", " businessos ", " work mode ", " work ", " client ", " clients ",
		" prd ", " github ", " ci ", " deploy ", " invoice ", " revenue ", " meeting ",
	}

	if explicit {
		for _, term := range lifeTerms[:3] {
			if strings.Contains(text, term) {
				return SurfaceLife, true
			}
		}
		for _, term := range businessTerms[:3] {
			if strings.Contains(text, term) {
				return SurfaceBusiness, true
			}
		}
		return "", false
	}

	lifeScore := countTerms(text, lifeTerms)
	businessScore := countTerms(text, businessTerms)
	switch {
	case lifeScore > businessScore:
		return SurfaceLife, true
	case businessScore > lifeScore:
		return SurfaceBusiness, true
	default:
		return "", false
	}
}

func inferSurfaceFromCalendar(events []CalendarEvent) (string, bool) {
	lifeScore := 0
	businessScore := 0
	for _, event := range events {
		text := strings.ToLower(event.Title + " " + event.Description)
		lifeScore += countTerms(" "+text+" ", []string{" family ", " kid ", " doctor ", " church ", " school ", " birthday ", " home "})
		businessScore += countTerms(" "+text+" ", []string{" client ", " work ", " standup ", " review ", " sales ", " project ", " prd "})
	}
	switch {
	case businessScore > lifeScore:
		return SurfaceBusiness, true
	case lifeScore > businessScore:
		return SurfaceLife, true
	default:
		return "", false
	}
}

func countTerms(text string, terms []string) int {
	count := 0
	for _, term := range terms {
		if strings.Contains(text, term) {
			count++
		}
	}
	return count
}

func normalizeSurfaceText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return ' '
	}, text)
	return " " + strings.Join(strings.Fields(text), " ") + " "
}

func isBusinessTime(at time.Time) bool {
	if at.IsZero() {
		at = time.Now()
	}
	weekday := at.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	hour := at.Hour()
	return hour >= 8 && hour < 17
}
