package db

import (
	"database/sql"
	"fmt"
)

// Helper: require that exactly one row was affected. Zero rows maps to
// sql.ErrNoRows so API handlers can return 404 via errors.Is.
func requireRowsAffected(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ScoredMemory is a memory with its FTS5 search relevance score.
type ScoredMemory struct {
	Memory
	FTSScore float64
}