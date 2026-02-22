package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

type IngestionFile struct {
	Path            string
	FileHash        string
	Status          string
	TotalChunks     int
	ProcessedChunks int
	LastError       sql.NullString
	UpdatedAt       string
	CompletedAt     sql.NullString
}

type IngestionChunk struct {
	ChunkHash  string
	FilePath   string
	FileHash   string
	ChunkIndex int
	Status     string
	MemoryID   sql.NullString
	Error      sql.NullString
	CreatedAt  string
	UpdatedAt  string
}

func (db *DB) IsIngestionChunkCompleted(chunkHash string) (bool, error) {
	chunkHash = strings.TrimSpace(chunkHash)
	if chunkHash == "" {
		return false, fmt.Errorf("chunk hash is required")
	}

	var status string
	err := db.conn.QueryRow(`SELECT status FROM ingestion_progress WHERE chunk_hash = ?`, chunkHash).Scan(&status)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(status), "completed"), nil
}

func (db *DB) UpsertIngestionChunk(c IngestionChunk) error {
	if strings.TrimSpace(c.ChunkHash) == "" {
		return fmt.Errorf("chunk hash is required")
	}
	if strings.TrimSpace(c.FilePath) == "" {
		return fmt.Errorf("file path is required")
	}
	if strings.TrimSpace(c.FileHash) == "" {
		return fmt.Errorf("file hash is required")
	}
	status := strings.ToLower(strings.TrimSpace(c.Status))
	if status == "" {
		status = "processing"
	}
	ts := now()
	createdAt := c.CreatedAt
	if createdAt == "" {
		createdAt = ts
	}

	_, err := db.conn.Exec(
		`INSERT INTO ingestion_progress (chunk_hash, file_path, file_hash, chunk_index, status, memory_id, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(chunk_hash) DO UPDATE SET
		   file_path = excluded.file_path,
		   file_hash = excluded.file_hash,
		   chunk_index = excluded.chunk_index,
		   status = excluded.status,
		   memory_id = excluded.memory_id,
		   error = excluded.error,
		   updated_at = excluded.updated_at`,
		c.ChunkHash, c.FilePath, c.FileHash, c.ChunkIndex, status, c.MemoryID, c.Error, createdAt, ts,
	)
	return err
}

func (db *DB) UpsertIngestionFile(f IngestionFile) error {
	if strings.TrimSpace(f.Path) == "" {
		return fmt.Errorf("ingestion file path is required")
	}
	if strings.TrimSpace(f.FileHash) == "" {
		return fmt.Errorf("ingestion file hash is required")
	}
	status := strings.ToLower(strings.TrimSpace(f.Status))
	if status == "" {
		status = "processing"
	}
	ts := now()

	var completedAt sql.NullString
	if status == "completed" {
		completedAt = sql.NullString{String: ts, Valid: true}
	}

	_, err := db.conn.Exec(
		`INSERT INTO ingestion_files (path, file_hash, status, total_chunks, processed_chunks, last_error, updated_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(path) DO UPDATE SET
		   file_hash = excluded.file_hash,
		   status = excluded.status,
		   total_chunks = excluded.total_chunks,
		   processed_chunks = excluded.processed_chunks,
		   last_error = excluded.last_error,
		   updated_at = excluded.updated_at,
		   completed_at = excluded.completed_at`,
		f.Path, f.FileHash, status, f.TotalChunks, f.ProcessedChunks, f.LastError, ts, completedAt,
	)
	return err
}

func (db *DB) ListIngestionFiles(limit int) ([]IngestionFile, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(
		`SELECT path, file_hash, status, total_chunks, processed_chunks, last_error, updated_at, completed_at
		 FROM ingestion_files
		 ORDER BY updated_at DESC
		 LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []IngestionFile
	for rows.Next() {
		var f IngestionFile
		if err := rows.Scan(
			&f.Path, &f.FileHash, &f.Status, &f.TotalChunks, &f.ProcessedChunks,
			&f.LastError, &f.UpdatedAt, &f.CompletedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
