package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type DashboardLast struct {
	Label      string    `json:"label"`
	Score      float64   `json:"score"`
	Confidence float64   `json:"confidence"`
	At         time.Time `json:"at"`
}

type DashboardTrend struct {
	Avg7d    float64 `json:"avg_7d"`
	Avg30d   float64 `json:"avg_30d"`
	Delta    float64 `json:"delta"`
	Count7d  int     `json:"count_7d"`
	Count30d int     `json:"count_30d"`
}

type DashboardSummary struct {
	Last  *DashboardLast `json:"last"`
	Trend DashboardTrend `json:"trend"`
}

type DashboardService struct {
	db *sqlx.DB
}

func NewDashboardService(db *sqlx.DB) *DashboardService {
	return &DashboardService{db: db}
}

func (s *DashboardService) Summary(ctx context.Context, userID uuid.UUID) (DashboardSummary, error) {
	var out DashboardSummary

	var last DashboardLast
	err := s.db.GetContext(ctx, &last, `
		SELECT ar.label, ar.score, ar.confidence, ar.created_at AS at
		FROM analysis_results ar
		JOIN analyses a ON a.id = ar.analysis_id
		WHERE a.user_id = $1
		ORDER BY ar.created_at DESC
		LIMIT 1
	`, userID)
	if err == nil {
		out.Last = &last
	} else if err != sql.ErrNoRows {
		return DashboardSummary{}, fmt.Errorf("latest result: %w", err)
	}

	type agg struct {
		Avg *float64 `db:"avg"`
		Cnt int      `db:"cnt"`
	}

	var a7 agg
	if err := s.db.GetContext(ctx, &a7, `
		SELECT AVG(ar.score) AS avg, COUNT(*) AS cnt
		FROM analysis_results ar
		JOIN analyses a ON a.id = ar.analysis_id
		WHERE a.user_id = $1 AND ar.created_at >= now() - interval '7 days'
	`, userID); err != nil {
		return DashboardSummary{}, fmt.Errorf("avg 7d: %w", err)
	}
	avg7 := 0.0
	if a7.Avg != nil {
		avg7 = *a7.Avg
	}

	var a30 agg
	if err := s.db.GetContext(ctx, &a30, `
		SELECT AVG(ar.score) AS avg, COUNT(*) AS cnt
		FROM analysis_results ar
		JOIN analyses a ON a.id = ar.analysis_id
		WHERE a.user_id = $1 AND ar.created_at >= now() - interval '30 days'
	`, userID); err != nil {
		return DashboardSummary{}, fmt.Errorf("avg 30d: %w", err)
	}

	avg30 := 0.0
	if a30.Avg != nil {
		avg30 = *a30.Avg
	}

	out.Trend = DashboardTrend{
		Avg7d:    avg7,
		Avg30d:   avg30,
		Delta:    avg7 - avg30,
		Count7d:  a7.Cnt,
		Count30d: a30.Cnt,
	}

	return out, nil
}
