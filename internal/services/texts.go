package services

import (
	"context"
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
