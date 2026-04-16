package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
)

type TextsRepo struct {
	db *sqlx.DB
}

func NewTextsRepo(db *sqlx.DB) *TextsRepo {
	return &TextsRepo{db: db}
}

func (r *TextsRepo) Create(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, content string) (models.Text, error) {
	var t models.Text
	if err := tx.GetContext(ctx, &t, `
		INSERT INTO texts (user_id, content)
		VALUES ($1, $2)
		RETURNING id, user_id, content, created_at
	`, userID, content); err != nil {
		return models.Text{}, fmt.Errorf("insert text: %w", err)
	}
	return t, nil
}

func (r *TextsRepo) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (models.Text, error) {
	var t models.Text
	if err := r.db.GetContext(ctx, &t, `
		SELECT id, user_id, content, created_at
		FROM texts
		WHERE id = $1 AND user_id = $2
	`, id, userID); err != nil {
		return models.Text{}, err
	}
	return t, nil
}

func (r *TextsRepo) GetByID(ctx context.Context, id uuid.UUID) (models.Text, error) {
	var t models.Text
	if err := r.db.GetContext(ctx, &t, `
		SELECT id, user_id, content, created_at
		FROM texts
		WHERE id = $1
	`, id); err != nil {
		return models.Text{}, err
	}
	return t, nil
}

func (r *TextsRepo) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Text, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM texts WHERE user_id = $1`, userID); err != nil {
		return nil, 0, fmt.Errorf("count texts: %w", err)
	}

	var items []models.Text
	if err := r.db.SelectContext(ctx, &items, `
		SELECT id, user_id, content, created_at
		FROM texts
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("list texts: %w", err)
	}

	return items, total, nil
}

func (r *TextsRepo) UpdateContentForUser(ctx context.Context, tx *sqlx.Tx, id, userID uuid.UUID, content string) (models.Text, error) {
	var t models.Text
	if err := tx.GetContext(ctx, &t, `
		UPDATE texts
		SET content = $3
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, content, created_at
	`, id, userID, content); err != nil {
		// Important: return sql.ErrNoRows as-is for service-level mapping to 404.
		return models.Text{}, err
	}
	return t, nil
}

func (r *TextsRepo) DeleteForUser(ctx context.Context, tx *sqlx.Tx, id, userID uuid.UUID) error {
	res, err := tx.ExecContext(ctx, `DELETE FROM texts WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete text: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete text rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
