package capture

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"scaffold/brain"
	"scaffold/db"
)

// Ingest runs the full capture→triage→memory pipeline.
// Returns the capture ID, memory ID (empty if triage failed), and triage result (may be nil on triage failure).
// A triage failure is non-fatal: capture is still stored and the ID returned.
// Memory insert + capture triage link is atomic when triage succeeds.
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

		var domainID sql.NullInt64
		resolvedName := strings.TrimSpace(triage.Domain)
		if resolvedName == "" {
			resolvedName = "Personal Development"
		}
		resolved, resolveErr := database.ResolveDomainID(resolvedName)
		if resolveErr != nil {
			log.Printf("triage: resolve domain %q: %v", resolvedName, resolveErr)
		} else if resolved != nil {
			domainID = sql.NullInt64{Int64: int64(*resolved), Valid: true}
		} else {
			log.Printf("triage: unknown domain %q, leaving undomained", resolvedName)
		}

		memoryID = uuid.New().String()
		mem := db.Memory{
			ID:         memoryID,
			Type:       triage.Type,
			Content:    text,
			Title:      triage.Title,
			Importance: triage.Importance,
			Source:     source,
			Tags:       strings.Join(triage.Tags, ","),
			DomainID:   domainID,
		}
		if err := database.PersistTriageResult(captureID, mem, triage.Action); err != nil {
			return captureID, "", triage, fmt.Errorf("persist triage result: %w", err)
		}
	}

	return captureID, memoryID, triage, nil
}
