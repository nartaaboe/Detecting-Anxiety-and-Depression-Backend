package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
)

type ResultsRepo struct {
	db *sqlx.DB
}

func NewResultsRepo(db *sqlx.DB) *ResultsRepo {
	return &ResultsRepo{db: db}
}

func (r *ResultsRepo) UpsertTx(ctx context.Context, tx *sqlx.Tx, analysisID uuid.UUID, label string, score, confidence float64, explanationJSON []byte) (models.AnalysisResult, error) {
	var res models.AnalysisResult
	if err := tx.GetContext(ctx, &res, `
		INSERT INTO analysis_results (analysis_id, label, score, confidence, explanation_json)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (analysis_id) DO UPDATE SET
			label = EXCLUDED.label,
			score = EXCLUDED.score,
			confidence = EXCLUDED.confidence,
			explanation_json = EXCLUDED.explanation_json,
			created_at = now()
		RETURNING id, analysis_id, label, score, confidence, explanation_json, created_at
	`, analysisID, label, score, confidence, explanationJSON); err != nil {
		return models.AnalysisResult{}, fmt.Errorf("upsert analysis result: %w", err)
	}
	return res, nil
}

func (r *ResultsRepo) Upsert(ctx context.Context, analysisID uuid.UUID, label string, score, confidence float64, explanationJSON []byte) (models.AnalysisResult, error) {
	var res models.AnalysisResult
	if err := r.db.GetContext(ctx, &res, `
		INSERT INTO analysis_results (analysis_id, label, score, confidence, explanation_json)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (analysis_id) DO UPDATE SET
			label = EXCLUDED.label,
			score = EXCLUDED.score,
			confidence = EXCLUDED.confidence,
			explanation_json = EXCLUDED.explanation_json,
			created_at = now()
		RETURNING id, analysis_id, label, score, confidence, explanation_json, created_at
	`, analysisID, label, score, confidence, explanationJSON); err != nil {
		return models.AnalysisResult{}, fmt.Errorf("upsert analysis result: %w", err)
	}
	return res, nil
}

func (r *ResultsRepo) GetByAnalysisID(ctx context.Context, analysisID uuid.UUID) (models.AnalysisResult, error) {
	var res models.AnalysisResult
	if err := r.db.GetContext(ctx, &res, `
		SELECT id, analysis_id, label, score, confidence, explanation_json, created_at
		FROM analysis_results
		WHERE analysis_id = $1
	`, analysisID); err != nil {
		return models.AnalysisResult{}, err
	}
	return res, nil
}

func (r *ResultsRepo) GetByAnalysisIDForUser(ctx context.Context, analysisID, userID uuid.UUID) (models.AnalysisResult, error) {
	var res models.AnalysisResult
	if err := r.db.GetContext(ctx, &res, `
		SELECT ar.id, ar.analysis_id, ar.label, ar.score, ar.confidence, ar.explanation_json, ar.created_at
		FROM analysis_results ar
		JOIN analyses a ON a.id = ar.analysis_id
		WHERE ar.analysis_id = $1 AND a.user_id = $2
	`, analysisID, userID); err != nil {
		return models.AnalysisResult{}, err
	}
	return res, nil
}
