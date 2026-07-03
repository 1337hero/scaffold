package db

import (
	"database/sql"
	"fmt"
)

// Helper: require that exactly one row was affected.
func requireRowsAffected(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("no rows affected")
	}
	return nil
}

// ScoredMemory is a memory with search relevance scores.
type ScoredMemory struct {
	Memory
	FTSScore    float64
	VectorScore float64
	FusedScore  float64
}