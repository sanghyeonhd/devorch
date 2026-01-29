package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Feedback: sqlite 테이블 행
type FeedbackRow struct {
	ID        string
	CreatedAt time.Time

	SessionID string
	MessageID string

	Provider string
	Model    string
	Scenario string

	ThumbsUp     bool
	Note         string
	QualityScore float64
}

type FeedbackStore struct {
	db *sql.DB
}

func NewFeedbackStore(db *sql.DB) *FeedbackStore {
	return &FeedbackStore{db: db}
}

// InsertFeedback implements FeedbackStore interface
func (s *FeedbackStore) InsertFeedback(ctx context.Context, f FeedbackRow) error {
	if s == nil || s.db == nil {
		return errors.New("sqlite: feedback db is nil")
	}
	if f.CreatedAt.IsZero() {
		f.CreatedAt = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO feedback(
  id, created_at,
  session_id, message_id,
  provider, model, scenario,
  thumbs_up, note,
  quality_score
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, f.ID, f.CreatedAt.UnixMilli(),
		f.SessionID, f.MessageID,
		f.Provider, f.Model, f.Scenario,
		boolToInt(f.ThumbsUp), f.Note,
		f.QualityScore,
	)
	return err
}
