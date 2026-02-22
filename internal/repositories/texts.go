package repositories

import (
	"context"
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
