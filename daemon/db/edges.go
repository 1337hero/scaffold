package db

type Edge struct {
	ID        string
	FromID    string
	ToID      string
	Relation  string
	Weight    float64
	CreatedAt string
}

func (db *DB) InsertEdge(e Edge) error {
	if e.ID == "" {
		e.ID = newID()
	}
	if e.CreatedAt == "" {
		e.CreatedAt = now()
	}
	if e.Weight == 0 {
		e.Weight = 1.0
	}

	_, err := db.conn.Exec(
		`INSERT INTO edges (id, from_id, to_id, relation, weight, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		e.ID, e.FromID, e.ToID, e.Relation, e.Weight, e.CreatedAt,
	)
	return err
}

func (db *DB) EdgesFrom(memoryID string) ([]Edge, error) {
	return db.queryEdges(
		`SELECT id, from_id, to_id, relation, weight, created_at FROM edges WHERE from_id = ?`, memoryID,
	)
}

func (db *DB) queryEdges(query string, args ...any) ([]Edge, error) {
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Edge
	for rows.Next() {
		var e Edge
		if err := rows.Scan(&e.ID, &e.FromID, &e.ToID, &e.Relation, &e.Weight, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
