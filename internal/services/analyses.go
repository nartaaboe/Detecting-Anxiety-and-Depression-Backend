package services

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/repositories"
)

type AnalysisQueue interface {
	Enqueue(analysisID uuid.UUID) error
}

type AnalysisService struct {
	db       *sqlx.DB
	texts    *repositories.TextsRepo
	analyses *repositories.AnalysesRepo
	results  *repositories.ResultsRepo
	settings *repositories.SettingsRepo
	queue    AnalysisQueue
}

func NewAnalysisService(
	db *sqlx.DB,
	texts *repositories.TextsRepo,
	analyses *repositories.AnalysesRepo,
	results *repositories.ResultsRepo,
	settings *repositories.SettingsRepo,
	queue AnalysisQueue,
) *AnalysisService {
	return &AnalysisService{
		db:       db,
		texts:    texts,
		analyses: analyses,
		results:  results,
		settings: settings,
		queue:    queue,
	}
}

type CreateAnalysisInput struct {
	TextID       *uuid.UUID
	Content      *string
	ModelVersion string
	Threshold    *float64
}

type CreateAnalysisResultInput struct {
	Label           string
	Score           float64
	Confidence      float64
	ExplanationJSON []byte
}

func (s *AnalysisService) Create(ctx context.Context, userID uuid.UUID, in CreateAnalysisInput) (models.Analysis, error) {
	var modelVersion string
	var threshold float64

	settings, err := s.settings.GetModelSettings(ctx)
	if err == nil {
		modelVersion = settings.DefaultModelVersion
		threshold = settings.DefaultThreshold
	} else {
		// If settings row is missing, fall back to request values (or defaults at handler-level).
		modelVersion = "baseline"
		threshold = 0.5
	}

	if mv := strings.TrimSpace(in.ModelVersion); mv != "" {
		modelVersion = mv
	}
	if in.Threshold != nil {
		threshold = *in.Threshold
	}
	if threshold <= 0 || threshold > 1 {
		return models.Analysis{}, fmt.Errorf("%w: threshold must be in (0,1]", ErrBadRequest)
	}

	var created models.Analysis

	err = repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		var textID uuid.UUID

		if in.TextID != nil {
			textID = *in.TextID
			var exists bool
			if err := tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM texts WHERE id = $1 AND user_id = $2)`, textID, userID); err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("%w: text not found", ErrNotFound)
			}
		} else if in.Content != nil {
			content := strings.TrimSpace(*in.Content)
			if content == "" {
				return fmt.Errorf("%w: content is required", ErrBadRequest)
			}
			t, err := s.texts.Create(ctx, tx, userID, content)
			if err != nil {
				return err
			}
			textID = t.ID
		} else {
			return fmt.Errorf("%w: text_id or content is required", ErrBadRequest)
		}

		a, err := s.analyses.Create(ctx, tx, userID, textID, modelVersion, threshold)
		if err != nil {
			if repositories.IsForeignKeyViolation(err) {
				return fmt.Errorf("%w: text not found", ErrNotFound)
			}
			return err
		}

		created = a
		return nil
	})
	if err != nil {
		return models.Analysis{}, err
	}

	if err := s.queue.Enqueue(created.ID); err != nil {
		_ = s.analyses.MarkFailed(context.Background(), created.ID, time.Now().UTC(), fmt.Sprintf("enqueue failed: %v", err))
		return models.Analysis{}, fmt.Errorf("%w: analysis queued but cannot be processed right now", ErrUnavailable)
	}

	return created, nil
}

func (s *AnalysisService) CreateWithResult(ctx context.Context, userID uuid.UUID, in CreateAnalysisInput, res CreateAnalysisResultInput) (models.Analysis, error) {
	label := strings.TrimSpace(res.Label)
	if label == "" {
		return models.Analysis{}, fmt.Errorf("%w: label is required", ErrBadRequest)
	}
	if res.Score < 0 || res.Score > 1 {
		return models.Analysis{}, fmt.Errorf("%w: score must be in [0,1]", ErrBadRequest)
	}
	if res.Confidence < 0 || res.Confidence > 1 {
		return models.Analysis{}, fmt.Errorf("%w: confidence must be in [0,1]", ErrBadRequest)
	}
	if len(res.ExplanationJSON) == 0 {
		return models.Analysis{}, fmt.Errorf("%w: explanation is required", ErrBadRequest)
	}

	var modelVersion string
	var threshold float64

	settings, err := s.settings.GetModelSettings(ctx)
	if err == nil {
		modelVersion = settings.DefaultModelVersion
		threshold = settings.DefaultThreshold
	} else {
		modelVersion = "baseline"
		threshold = 0.5
	}

	if mv := strings.TrimSpace(in.ModelVersion); mv != "" {
		modelVersion = mv
	}
	if in.Threshold != nil {
		threshold = *in.Threshold
	}
	if threshold <= 0 || threshold > 1 {
		return models.Analysis{}, fmt.Errorf("%w: threshold must be in (0,1]", ErrBadRequest)
	}

	now := time.Now().UTC()

	var created models.Analysis

	err = repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		var textID uuid.UUID

		if in.TextID != nil {
			textID = *in.TextID
			var exists bool
			if err := tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM texts WHERE id = $1 AND user_id = $2)`, textID, userID); err != nil {
				return err
			}
			if !exists {
				return fmt.Errorf("%w: text not found", ErrNotFound)
			}
		} else if in.Content != nil {
			content := strings.TrimSpace(*in.Content)
			if content == "" {
				return fmt.Errorf("%w: content is required", ErrBadRequest)
			}
			t, err := s.texts.Create(ctx, tx, userID, content)
			if err != nil {
				return err
			}
			textID = t.ID
		} else {
			return fmt.Errorf("%w: text_id or content is required", ErrBadRequest)
		}

		a, err := s.analyses.Create(ctx, tx, userID, textID, modelVersion, threshold)
		if err != nil {
			if repositories.IsForeignKeyViolation(err) {
				return fmt.Errorf("%w: text not found", ErrNotFound)
			}
			return err
		}

		if _, err := s.results.UpsertTx(ctx, tx, a.ID, label, res.Score, res.Confidence, res.ExplanationJSON); err != nil {
			return err
		}

		if err := s.analyses.MarkDoneTx(ctx, tx, a.ID, now); err != nil {
			return err
		}

		a.Status = models.AnalysisDone
		a.FinishedAt = &now
		created = a
		return nil
	})
	if err != nil {
		return models.Analysis{}, err
	}

	return created, nil
}

func (s *AnalysisService) Get(ctx context.Context, userID, analysisID uuid.UUID) (models.Analysis, error) {
	a, err := s.analyses.GetByIDForUser(ctx, analysisID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Analysis{}, fmt.Errorf("%w: analysis not found", ErrNotFound)
		}
		return models.Analysis{}, err
	}
	text, err := s.texts.GetByID(ctx, a.TextID)
	if err != nil {
		return models.Analysis{}, err
	}
	a.TextContent = text.Content

	return a, nil
}

func (s *AnalysisService) List(ctx context.Context, userID uuid.UUID, f repositories.AnalysisListFilter) ([]models.AnalysisListItem, int, error) {
	items, total, err := s.analyses.ListForUser(ctx, userID, f)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		text, err := s.texts.GetByID(ctx, items[i].TextID)
		if err != nil {
			continue
		}
		items[i].TextContent = text.Content
	}
	return items, total, nil
}

func (s *AnalysisService) GetResult(ctx context.Context, userID, analysisID uuid.UUID) (models.AnalysisResult, error) {
	res, err := s.results.GetByAnalysisIDForUser(ctx, analysisID, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.AnalysisResult{}, fmt.Errorf("%w: result not found", ErrNotFound)
		}
		return models.AnalysisResult{}, err
	}
	return res, nil
}

func (s *AnalysisService) UpsertResult(ctx context.Context, userID, analysisID uuid.UUID, in CreateAnalysisResultInput) (models.Analysis, error) {
	label := strings.TrimSpace(in.Label)
	if label == "" {
		return models.Analysis{}, fmt.Errorf("%w: label is required", ErrBadRequest)
	}
	if in.Score < 0 || in.Score > 1 {
		return models.Analysis{}, fmt.Errorf("%w: score must be in [0,1]", ErrBadRequest)
	}
	if in.Confidence < 0 || in.Confidence > 1 {
		return models.Analysis{}, fmt.Errorf("%w: confidence must be in [0,1]", ErrBadRequest)
	}
	if len(in.ExplanationJSON) == 0 {
		return models.Analysis{}, fmt.Errorf("%w: explanation is required", ErrBadRequest)
	}

	now := time.Now().UTC()

	var updated models.Analysis
	err := repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		// Ensure analysis exists and belongs to the user.
		a, err := s.analyses.GetByIDForUser(ctx, analysisID, userID)
		if err != nil {
			return err
		}

		if _, err := s.results.UpsertTx(ctx, tx, analysisID, label, in.Score, in.Confidence, in.ExplanationJSON); err != nil {
			return err
		}

		if err := s.analyses.MarkDoneForUserTx(ctx, tx, analysisID, userID, now); err != nil {
			return err
		}

		a.Status = models.AnalysisDone
		a.FinishedAt = &now
		a.ErrorMessage = nil
		updated = a
		return nil
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return models.Analysis{}, fmt.Errorf("%w: analysis not found", ErrNotFound)
		}
		return models.Analysis{}, err
	}

	return updated, nil
}

func (s *AnalysisService) Delete(ctx context.Context, userID, analysisID uuid.UUID) error {
	err := repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		return s.analyses.DeleteForUserTx(ctx, tx, analysisID, userID)
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: analysis not found", ErrNotFound)
		}
		return err
	}
	return nil
}
