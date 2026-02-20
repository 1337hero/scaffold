package cron

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"time"

	"scaffold/brain"
	"scaffold/db"
)

func Start(ctx context.Context, database *db.DB, b *brain.Brain) {
	go runAt(ctx, 7, func() { runPrioritization(ctx, database, b) })
	go runAt(ctx, 3, func() {
		if err := database.CleanExpiredSessions(); err != nil {
			log.Printf("cron: clean sessions: %v", err)
		} else {
			log.Println("cron: expired sessions cleaned")
		}
	})
}

func runAt(ctx context.Context, hour int, fn func()) {
	for {
		next := nextOccurrence(hour)
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			fn()
		}
	}
}

func nextOccurrence(hour int) time.Time {
	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, 0, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

func runPrioritization(ctx context.Context, database *db.DB, b *brain.Brain) {
	log.Println("cron: starting morning prioritization")

	todos, err := database.ListTodosByImportance(0.5, 20)
	if err != nil {
		log.Printf("cron: list todos: %v", err)
		return
	}

	yesterdayDesk, err := database.YesterdaysDesk()
	if err != nil {
		log.Printf("cron: yesterday's desk: %v", err)
		return
	}

	tasks, err := b.Prioritize(ctx, todos, yesterdayDesk)
	if err != nil {
		log.Printf("cron: prioritize: %v", err)
		return
	}

	today := time.Now().Format("2006-01-02")
	for i, task := range tasks {
		stepsJSON, err := json.Marshal(task.MicroSteps)
		if err != nil {
			log.Printf("cron: marshal micro_steps for %q: %v", task.Title, err)
			continue
		}

		item := db.DeskItem{
			Title:    task.Title,
			Position: i + 1,
			Status:   "active",
			MicroSteps: sql.NullString{
				String: string(stepsJSON),
				Valid:  true,
			},
			Date: today,
		}
		if task.SourceMemoryID != "" {
			item.MemoryID = sql.NullString{
				String: task.SourceMemoryID,
				Valid:  true,
			}
		}

		if err := database.InsertDeskItem(item); err != nil {
			log.Printf("cron: insert desk item %q: %v", task.Title, err)
		} else {
			log.Printf("cron: desk[%d] %q — %s", i+1, task.Title, task.Why)
		}
	}

	log.Printf("cron: prioritization complete — %d tasks on desk", len(tasks))
}
