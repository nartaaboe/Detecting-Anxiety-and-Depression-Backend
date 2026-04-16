package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
)

type AnalysisListFilter struct {
	Status string
	Label  string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

type AnalysesRepo struct {
	db *sqlx.DB
}

func NewAnalysesRepo(db *sqlx.DB) *AnalysesRepo {
	return &AnalysesRepo{db: db}
}

func (r *AnalysesRepo) Create(ctx context.Context, tx *sqlx.Tx, userID, textID uuid.UUID, modelVersion string, threshold float64) (models.Analysis, error) {
	var a models.Analysis
	if err := tx.GetContext(ctx, &a, `
		INSERT INTO analyses (user_id, text_id, status, model_version, threshold)
		VALUES ($1, $2, 'queued', $3, $4)
		RETURNING id, user_id, text_id, status, model_version, threshold, created_at, started_at, finished_at, error_message
	`, userID, textID, modelVersion, threshold); err != nil {
		return models.Analysis{}, fmt.Errorf("insert analysis: %w", err)
	}
	return a, nil
}

func (r *AnalysesRepo) GetByID(ctx context.Context, id uuid.UUID) (models.Analysis, error) {
	var a models.Analysis
	if err := r.db.GetContext(ctx, &a, `
		SELECT id, user_id, text_id, status, model_version, threshold, created_at, started_at, finished_at, error_message
		FROM analyses
		WHERE id = $1
	`, id); err != nil {
		return models.Analysis{}, err
	}
	return a, nil
}

func (r *AnalysesRepo) GetByIDForUser(ctx context.Context, id, userID uuid.UUID) (models.Analysis, error) {
	var a models.Analysis
	if err := r.db.GetContext(ctx, &a, `
		SELECT id, user_id, text_id, status, model_version, threshold, created_at, started_at, finished_at, error_message
		FROM analyses
		WHERE id = $1 AND user_id = $2
	`, id, userID); err != nil {
		return models.Analysis{}, err
	}
	return a, nil
}

func (r *AnalysesRepo) ListForUser(ctx context.Context, userID uuid.UUID, f AnalysisListFilter) ([]models.AnalysisListItem, int, error) {
	where, args := buildAnalysisWhere(true, userID, f)

	var total int
	if err := r.db.GetContext(ctx, &total, `
		SELECT COUNT(*)
		FROM analyses a
		LEFT JOIN analysis_results ar ON ar.analysis_id = a.id
	`+where, args...); err != nil {
		return nil, 0, fmt.Errorf("count analyses: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	limitPos := len(args) - 1
	offsetPos := len(args)

	query := fmt.Sprintf(`
		SELECT
			a.id, a.user_id, a.text_id, a.status, a.model_version, a.threshold,
			a.created_at, a.started_at, a.finished_at, a.error_message,
			ar.label AS result_label,
			ar.score AS result_score,
			ar.confidence AS result_confidence
		FROM analyses a
		LEFT JOIN analysis_results ar ON ar.analysis_id = a.id
		%s
		ORDER BY a.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, limitPos, offsetPos)

	var items []models.AnalysisListItem
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list analyses: %w", err)
	}
	return items, total, nil
}

func (r *AnalysesRepo) ListAll(ctx context.Context, f AnalysisListFilter) ([]models.AdminAnalysisListItem, int, error) {
	where, args := buildAnalysisWhere(false, uuid.UUID{}, f)

	var total int
	if err := r.db.GetContext(ctx, &total, `
		SELECT COUNT(*)
		FROM analyses a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN analysis_results ar ON ar.analysis_id = a.id
	`+where, args...); err != nil {
		return nil, 0, fmt.Errorf("count analyses: %w", err)
	}

	args = append(args, f.Limit, f.Offset)
	limitPos := len(args) - 1
	offsetPos := len(args)

	query := fmt.Sprintf(`
		SELECT
			a.id, a.user_id, a.text_id, a.status, a.model_version, a.threshold,
			a.created_at, a.started_at, a.finished_at, a.error_message,
			ar.label AS result_label,
			ar.score AS result_score,
			ar.confidence AS result_confidence,
			u.email AS user_email
		FROM analyses a
		JOIN users u ON u.id = a.user_id
		LEFT JOIN analysis_results ar ON ar.analysis_id = a.id
		%s
		ORDER BY a.created_at DESC
		LIMIT $%d OFFSET $%d
	`, where, limitPos, offsetPos)

	var items []models.AdminAnalysisListItem
	if err := r.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, 0, fmt.Errorf("list analyses: %w", err)
	}
	return items, total, nil
}

func (r *AnalysesRepo) MarkRunning(ctx context.Context, id uuid.UUID, now time.Time) (models.Analysis, bool, error) {
	var a models.Analysis
	err := r.db.GetContext(ctx, &a, `
		UPDATE analyses
		SET status = 'running', started_at = $2
		WHERE id = $1 AND status = 'queued'
		RETURNING id, user_id, text_id, status, model_version, threshold, created_at, started_at, finished_at, error_message
	`, id, now)
	if err == nil {
		return a, true, nil
	}
	if err == sql.ErrNoRows {
		return models.Analysis{}, false, nil
	}
	return models.Analysis{}, false, fmt.Errorf("mark running: %w", err)
}

func (r *AnalysesRepo) MarkDone(ctx context.Context, id uuid.UUID, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE analyses
		SET status = 'done', finished_at = $2, error_message = NULL
		WHERE id = $1
	`, id, now)
	if err != nil {
		return fmt.Errorf("mark done: %w", err)
	}
	return nil
}

func (r *AnalysesRepo) MarkDoneTx(ctx context.Context, tx *sqlx.Tx, id uuid.UUID, now time.Time) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE analyses
		SET status = 'done', finished_at = $2, error_message = NULL
		WHERE id = $1
	`, id, now)
	if err != nil {
		return fmt.Errorf("mark done: %w", err)
	}
	return nil
}

func (r *AnalysesRepo) MarkDoneForUserTx(ctx context.Context, tx *sqlx.Tx, id, userID uuid.UUID, now time.Time) error {
	res, err := tx.ExecContext(ctx, `
		UPDATE analyses
		SET status = 'done', finished_at = $3, error_message = NULL
		WHERE id = $1 AND user_id = $2
	`, id, userID, now)
	if err != nil {
		return fmt.Errorf("mark done: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("mark done rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AnalysesRepo) MarkFailed(ctx context.Context, id uuid.UUID, now time.Time, msg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE analyses
		SET status = 'failed', finished_at = $2, error_message = $3
		WHERE id = $1
	`, id, now, msg)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

func (r *AnalysesRepo) DeleteForUserTx(ctx context.Context, tx *sqlx.Tx, id, userID uuid.UUID) error {
	res, err := tx.ExecContext(ctx, `DELETE FROM analyses WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete analysis: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete analysis rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *AnalysesRepo) GetTextContentForAnalysis(ctx context.Context, analysisID uuid.UUID) (string, error) {
	var content string
	if err := r.db.GetContext(ctx, &content, `
		SELECT t.content
		FROM analyses a
		JOIN texts t ON t.id = a.text_id
		WHERE a.id = $1
	`, analysisID); err != nil {
		return "", err
	}
	return content, nil
}

func buildAnalysisWhere(requireUser bool, userID uuid.UUID, f AnalysisListFilter) (string, []any) {
	conds := make([]string, 0, 8)
	args := make([]any, 0, 8)

	argN := 1
	if requireUser {
		conds = append(conds, fmt.Sprintf("a.user_id = $%d", argN))
		args = append(args, userID)
		argN++
	}

	if f.Status != "" {
		conds = append(conds, fmt.Sprintf("a.status = $%d", argN))
		args = append(args, f.Status)
		argN++
	}
	if f.Label != "" {
		conds = append(conds, fmt.Sprintf("ar.label = $%d", argN))
		args = append(args, f.Label)
		argN++
	}
	if f.From != nil {
		conds = append(conds, fmt.Sprintf("a.created_at >= $%d", argN))
		args = append(args, *f.From)
		argN++
	}
	if f.To != nil {
		conds = append(conds, fmt.Sprintf("a.created_at <= $%d", argN))
		args = append(args, *f.To)
		argN++
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}
