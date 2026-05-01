package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/session"
)

type SessionStore struct {
	db *sql.DB
}

func NewSessionStore(db *sql.DB) *SessionStore {
	return &SessionStore{db: db}
}

func (s *SessionStore) GetOrCreate(ctx context.Context, channel, openID string, now time.Time) (*session.Session, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	var sess session.Session
	err := s.db.QueryRowContext(ctx, `SELECT id::text, channel, open_id, COALESCE(user_id,''), status, turn_count, last_message_at, created_at, updated_at FROM cs_sessions WHERE channel = $1 AND open_id = $2 AND status != 'closed' ORDER BY updated_at DESC LIMIT 1`, channel, openID).Scan(&sess.ID, &sess.Channel, &sess.OpenID, &sess.UserID, &sess.Status, &sess.TurnCount, &sess.LastMessageAt, new(time.Time), new(time.Time))
	if err == nil {
		return &sess, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}
	err = s.db.QueryRowContext(ctx, `INSERT INTO cs_sessions(channel, open_id, status, turn_count, last_message_at) VALUES ($1,$2,'idle',0,$3) RETURNING id::text, channel, open_id, COALESCE(user_id,''), status, turn_count, last_message_at, created_at, updated_at`, channel, openID, now).Scan(&sess.ID, &sess.Channel, &sess.OpenID, &sess.UserID, &sess.Status, &sess.TurnCount, &sess.LastMessageAt, new(time.Time), new(time.Time))
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *SessionStore) GetByID(ctx context.Context, id string) (*session.Session, error) {
	if s.db == nil {
		return nil, fmt.Errorf("db is nil")
	}
	var sess session.Session
	err := s.db.QueryRowContext(ctx,
		`SELECT id::text, channel, open_id, COALESCE(user_id,''), status, turn_count, last_message_at, created_at, updated_at FROM cs_sessions WHERE id = $1::uuid`,
		id,
	).Scan(&sess.ID, &sess.Channel, &sess.OpenID, &sess.UserID, &sess.Status, &sess.TurnCount, &sess.LastMessageAt, new(time.Time), new(time.Time))
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *SessionStore) Save(ctx context.Context, sess *session.Session) error {
	if s.db == nil {
		return fmt.Errorf("db is nil")
	}
	_, err := s.db.ExecContext(ctx, `UPDATE cs_sessions SET user_id = NULLIF($2,''), status = $3, turn_count = $4, last_message_at = $5, updated_at = NOW() WHERE id = $1::uuid`, sess.ID, sess.UserID, string(sess.Status), sess.TurnCount, sess.LastMessageAt)
	return err
}
