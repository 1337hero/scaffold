package agentprompt

import (
	"strings"
	"testing"
	"time"
)

func TestAssembleSystemPromptIncludesLayers(t *testing.T) {
	prompt := AssembleSystemPrompt(Identity{
		Name:     "Scaffold",
		UserName: "Mike",
		Voice:    []string{"Direct, warm, concise."},
		Values:   []string{"Witness without deciding what matters."},
		Posture:  []string{"Push back when the pattern is visible."},
		CannotDo: []string{"Access email.", "Run code."},
		Rules:    []string{"Keep Signal replies tight."},
	}, "Bulletin: the fence project keeps slipping.", SurfaceLife, []Fact{{
		Entity:   "Mike",
		Content:  "Mike prefers short closeouts.",
		Category: "user_pref",
		Trust:    0.85,
	}})

	for _, want := range []string{
		"You are Scaffold, Mike's personal agent.",
		"## Voice",
		"Direct, warm, concise.",
		"## Current Surface",
		"LifeOS",
		"the fence project keeps slipping",
		"## High-Trust Facts",
		"Mike prefers short closeouts",
		"## Cannot Do",
		"Access email.",
		"## Operating Rules",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestDetectSurface(t *testing.T) {
	businessTime := time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	evening := time.Date(2026, 7, 6, 19, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		explicit string
		at       time.Time
		events   []CalendarEvent
		query    string
		want     string
	}{
		{
			name:     "explicit switch overrides time",
			explicit: "switch to life mode",
			at:       businessTime,
			events:   []CalendarEvent{{Title: "Client review"}},
			query:    "revenue plan",
			want:     SurfaceLife,
		},
		{
			name: "business hours fallback",
			at:   businessTime,
			want: SurfaceBusiness,
		},
		{
			name:   "calendar context",
			at:     evening,
			events: []CalendarEvent{{Title: "Client launch review"}},
			want:   SurfaceBusiness,
		},
		{
			name:  "query inference",
			at:    evening,
			query: "what did I say about the fence?",
			want:  SurfaceLife,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectSurface(tt.explicit, tt.at, tt.events, tt.query); got != tt.want {
				t.Fatalf("DetectSurface=%q, want %q", got, tt.want)
			}
		})
	}
}
