package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
)

type SessionsRepo struct {
	db *sqlx.DB
}

func NewSessionsRepo(db *sqlx.DB) *SessionsRepo {
	return &SessionsRepo{db: db}
}

func (r *SessionsRepo) Create(ctx context.Context, tx *sqlx.Tx, sessionID uuid.UUID, userID uuid.UUID, refreshTokenHash string, expiresAt time.Time) (models.UserSession, error) {
	var s models.UserSession
	if err := tx.GetContext(ctx, &s, `
		INSERT INTO user_sessions (id, user_id, refresh_token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, refresh_token_hash, expires_at, created_at, revoked_at
	`, sessionID, userID, refreshTokenHash, expiresAt); err != nil {
		return models.UserSession{}, fmt.Errorf("insert session: %w", err)
	}
	return s, nil
}

func (r *SessionsRepo) GetByID(ctx context.Context, id uuid.UUID) (models.UserSession, error) {
	var s models.UserSession
	if err := r.db.GetContext(ctx, &s, `
		SELECT id, user_id, refresh_token_hash, expires_at, created_at, revoked_at
		FROM user_sessions
		WHERE id = $1
	`, id); err != nil {
		return models.UserSession{}, err
	}
	return s, nil
}

func (r *SessionsRepo) Revoke(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, now time.Time) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE user_sessions
		SET revoked_at = $2
		WHERE id = $1 AND revoked_at IS NULL
	`, id, now)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("revoke session rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *SessionsRepo) RevokeAllForUser(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE user_sessions
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, now)
	if err != nil {
		return fmt.Errorf("revoke all sessions: %w", err)
	}
	return nil
}
