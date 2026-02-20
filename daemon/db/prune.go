package db

import (
	"fmt"
	"time"
)

type PruneReport struct {
	Candidates         int
	Deleted            int
	SkippedActiveEdges int
	SkippedReferences  int
	EdgeRowsDeleted    int
}

// PruneSuppressedMemories permanently deletes memories that meet all guards:
// - suppressed for at least suppressedDays
// - no active edges to unsuppressed memories
// - not referenced by desk or captures
func (db *DB) PruneSuppressedMemories(suppressedDays int) (PruneReport, error) {
	if suppressedDays <= 0 {
		suppressedDays = 30
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -suppressedDays).Format(time.RFC3339)
	rows, err := db.conn.Query(
		`SELECT id
		 FROM memories
		 WHERE suppressed_at IS NOT NULL
		   AND suppressed_at <= ?
		 ORDER BY suppressed_at ASC`,
		cutoff,
	)
	if err != nil {
		return PruneReport{}, fmt.Errorf("query prune candidates: %w", err)
	}
	defer rows.Close()

	candidates := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return PruneReport{}, fmt.Errorf("scan prune candidate: %w", err)
		}
		candidates = append(candidates, id)
	}
	if err := rows.Err(); err != nil {
		return PruneReport{}, fmt.Errorf("iterate prune candidates: %w", err)
	}

	report := PruneReport{}
	for _, memoryID := range candidates {
		report.Candidates++

		referenceCount, err := db.countPruneReferences(memoryID)
		if err != nil {
			return report, fmt.Errorf("count references for %s: %w", memoryID, err)
		}
		if referenceCount > 0 {
			report.SkippedReferences++
			continue
		}

		activeEdges, err := db.countActiveEdges(memoryID)
		if err != nil {
			return report, fmt.Errorf("count active edges for %s: %w", memoryID, err)
		}
		if activeEdges > 0 {
			report.SkippedActiveEdges++
			continue
		}

		edgeRowsDeleted, err := db.deleteMemoryAndEdges(memoryID)
		if err != nil {
			return report, fmt.Errorf("delete memory %s: %w", memoryID, err)
		}
		report.Deleted++
		report.EdgeRowsDeleted += edgeRowsDeleted
	}

	return report, nil
}

func (db *DB) CountMemoryReferences(id string) (int, error) {
	return db.countPruneReferences(id)
}

func (db *DB) countPruneReferences(memoryID string) (int, error) {
	var deskRefs int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM desk WHERE memory_id = ?`, memoryID).Scan(&deskRefs); err != nil {
		return 0, err
	}

	var captureRefs int
	if err := db.conn.QueryRow(`SELECT COUNT(*) FROM captures WHERE memory_id = ?`, memoryID).Scan(&captureRefs); err != nil {
		return 0, err
	}

	return deskRefs + captureRefs, nil
}

func (db *DB) countActiveEdges(memoryID string) (int, error) {
	var count int
	err := db.conn.QueryRow(
		`SELECT COUNT(*)
		 FROM edges e
		 LEFT JOIN memories other
		   ON other.id = CASE
		                 WHEN e.from_id = ? THEN e.to_id
		                 ELSE e.from_id
		               END
		 WHERE (e.from_id = ? OR e.to_id = ?)
		   AND (other.id IS NULL OR other.suppressed_at IS NULL)`,
		memoryID, memoryID, memoryID,
	).Scan(&count)
	return count, err
}

func (db *DB) deleteMemoryAndEdges(memoryID string) (int, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	edgeDeleteResult, err := tx.Exec(`DELETE FROM edges WHERE from_id = ? OR to_id = ?`, memoryID, memoryID)
	if err != nil {
		return 0, err
	}
	edgeRows, err := edgeDeleteResult.RowsAffected()
	if err != nil {
		return 0, err
	}

	memoryDeleteResult, err := tx.Exec(`DELETE FROM memories WHERE id = ?`, memoryID)
	if err != nil {
		return 0, err
	}
	if err := requireRowsAffected(memoryDeleteResult); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return int(edgeRows), nil
}
