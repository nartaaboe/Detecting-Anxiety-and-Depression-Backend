package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/repositories"
)

type TextService struct {
	db    *sqlx.DB
	texts *repositories.TextsRepo
}

func NewTextService(db *sqlx.DB, texts *repositories.TextsRepo) *TextService {
	return &TextService{db: db, texts: texts}
}

func (s *TextService) Create(ctx context.Context, userID uuid.UUID, content string) (models.Text, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return models.Text{}, fmt.Errorf("%w: content is required", ErrBadRequest)
	}

	var out models.Text
	err := repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		t, err := s.texts.Create(ctx, tx, userID, content)
		if err != nil {
			return err
		}
		out = t
		return nil
	})
	if err != nil {
		return models.Text{}, err
	}
	return out, nil
}

func (s *TextService) Get(ctx context.Context, userID, textID uuid.UUID) (models.Text, error) {
	t, err := s.texts.GetByIDForUser(ctx, textID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Text{}, fmt.Errorf("%w: text not found", ErrNotFound)
		}
		return models.Text{}, err
	}
	return t, nil
}

func (s *TextService) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Text, int, error) {
	items, total, err := s.texts.ListForUser(ctx, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *TextService) Update(ctx context.Context, userID, textID uuid.UUID, content string) (models.Text, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return models.Text{}, fmt.Errorf("%w: content is required", ErrBadRequest)
	}

	var out models.Text
	err := repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		t, err := s.texts.UpdateContentForUser(ctx, tx, textID, userID, content)
		if err != nil {
			return err
		}
		out = t
		return nil
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Text{}, fmt.Errorf("%w: text not found", ErrNotFound)
		}
		return models.Text{}, err
	}
	return out, nil
}

func (s *TextService) Delete(ctx context.Context, userID, textID uuid.UUID) error {
	err := repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		return s.texts.DeleteForUser(ctx, tx, textID, userID)
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: text not found", ErrNotFound)
		}
		return err
	}
	return nil
}
