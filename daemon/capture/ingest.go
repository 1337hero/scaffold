package capture

import (
	"context"
	"log"
	"strings"

	"github.com/google/uuid"

	"scaffold/brain"
	"scaffold/db"
)

// Ingest runs the full capture→triage→memory pipeline.
// Returns the capture ID, memory ID (empty if triage failed), and triage result (may be nil on triage failure).
// A triage failure is non-fatal: capture is still stored and the ID returned.
func Ingest(ctx context.Context, database *db.DB, b *brain.Brain, text, source string) (captureID, memoryID string, triage *brain.TriageResult, err error) {
	captureID, err = database.InsertCapture(text, source)
	if err != nil {
		return "", "", nil, err
	}

	if b != nil {
		var triageErr error
		triage, triageErr = b.Triage(ctx, text)
		if triageErr != nil {
			log.Printf("triage error: %v", triageErr)
		}
	}

	if triage != nil {
		log.Printf("triage: type=%s action=%s importance=%.1f", triage.Type, triage.Action, triage.Importance)

		memoryID = uuid.New().String()
		mem := db.Memory{
			ID:         memoryID,
			Type:       triage.Type,
			Content:    text,
			Title:      triage.Title,
			Importance: triage.Importance,
			Source:     source,
			Tags:       strings.Join(triage.Tags, ","),
		}
		if err := database.InsertMemory(mem); err != nil {
			log.Printf("memory insert error: %v", err)
		}

		if err := database.UpdateTriage(captureID, triage.Action, memoryID); err != nil {
			log.Printf("triage update error: %v", err)
		}
	}

	return captureID, memoryID, triage, nil
}
