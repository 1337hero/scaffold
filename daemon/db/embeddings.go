package db

import (
	"encoding/binary"
	"math"
	"sort"
)

type ScoredMemory struct {
	Memory
	FTSScore    float64
	VectorScore float64
	FusedScore  float64
}

func (db *DB) UpsertEmbedding(memoryID string, embedding []float32, model string) error {
	_, err := db.conn.Exec(
		`INSERT INTO memory_embeddings (memory_id, embedding, model, updated_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(memory_id) DO UPDATE SET embedding = excluded.embedding, model = excluded.model, updated_at = excluded.updated_at`,
		memoryID, float32ToBytes(embedding), model, now(),
	)
	return err
}

func (db *DB) GetEmbedding(memoryID string) ([]float32, error) {
	var blob []byte
	err := db.conn.QueryRow(`SELECT embedding FROM memory_embeddings WHERE memory_id = ?`, memoryID).Scan(&blob)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return bytesToFloat32(blob), nil
}

func (db *DB) ListEmbeddings() (map[string][]float32, error) {
	rows, err := db.conn.Query(`SELECT memory_id, embedding FROM memory_embeddings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string][]float32)
	for rows.Next() {
		var id string
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, err
		}
		out[id] = bytesToFloat32(blob)
	}
	return out, rows.Err()
}

func (db *DB) NearestNeighbors(target []float32, topK int, excludeIDs []string) ([]ScoredMemory, error) {
	embeddings, err := db.ListEmbeddings()
	if err != nil {
		return nil, err
	}

	excluded := make(map[string]bool, len(excludeIDs))
	for _, id := range excludeIDs {
		excluded[id] = true
	}

	type scored struct {
		id    string
		score float64
	}
	var candidates []scored
	for id, emb := range embeddings {
		if excluded[id] {
			continue
		}
		candidates = append(candidates, scored{id, cosineSimilarity(target, emb)})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}

	var results []ScoredMemory
	for _, c := range candidates {
		mem, err := db.GetMemory(c.id)
		if err != nil {
			return nil, err
		}
		if mem == nil {
			continue
		}
		results = append(results, ScoredMemory{
			Memory:      *mem,
			VectorScore: c.score,
		})
	}
	return results, nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func float32ToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func bytesToFloat32(b []byte) []float32 {
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}
