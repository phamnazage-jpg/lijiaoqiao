package postgres

import (
	"context"
	"database/sql"
	"fmt"
)

type DedupStore struct {
	db *sql.DB
}

func NewDedupStore(db *sql.DB) *DedupStore {
	return &DedupStore{db: db}
}

func (s *DedupStore) TryRecord(ctx context.Context, channel, messageID, sessionID string) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("db is nil")
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO cs_message_dedup(channel, message_id, session_id) VALUES ($1,$2,NULLIF($3,'')::uuid) ON CONFLICT DO NOTHING`, channel, messageID, sessionID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}
