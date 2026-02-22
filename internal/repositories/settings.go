package repositories

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
)

type SettingsRepo struct {
	db *sqlx.DB
}

func NewSettingsRepo(db *sqlx.DB) *SettingsRepo {
	return &SettingsRepo{db: db}
}

func (r *SettingsRepo) EnsureModelSettings(ctx context.Context, defaultModelVersion string, defaultThreshold float64) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO model_settings (id, default_model_version, default_threshold)
		VALUES (1, $1, $2)
		ON CONFLICT (id) DO NOTHING
	`, defaultModelVersion, defaultThreshold)
	if err != nil {
		return fmt.Errorf("ensure model settings: %w", err)
	}
	return nil
}

func (r *SettingsRepo) GetModelSettings(ctx context.Context) (models.ModelSettings, error) {
	var s models.ModelSettings
	if err := r.db.GetContext(ctx, &s, `
		SELECT id, default_model_version, default_threshold, updated_at
		FROM model_settings
		WHERE id = 1
	`); err != nil {
		return models.ModelSettings{}, err
	}
	return s, nil
}

func (r *SettingsRepo) UpdateModelSettings(ctx context.Context, tx *sqlx.Tx, defaultModelVersion string, defaultThreshold float64) (models.ModelSettings, error) {
	var s models.ModelSettings
	if err := tx.GetContext(ctx, &s, `
		UPDATE model_settings
		SET default_model_version = $1, default_threshold = $2, updated_at = now()
		WHERE id = 1
		RETURNING id, default_model_version, default_threshold, updated_at
	`, defaultModelVersion, defaultThreshold); err != nil {
		return models.ModelSettings{}, fmt.Errorf("update model settings: %w", err)
	}
	return s, nil
}
