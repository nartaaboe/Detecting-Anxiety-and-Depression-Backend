package services

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type UserStats struct {
	Total      int     `json:"total"`
	HighRisk   int     `json:"high_risk"`
	MediumRisk int     `json:"medium_risk"`
	LowRisk    int     `json:"low_risk"`
	AvgConf    float64 `json:"avg_confidence"`
	ThisWeek   int     `json:"this_week"`
}

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

type ExportAnalysis struct {
	ID           string    `db:"id" json:"id"`
	TextContent  string    `db:"text_content" json:"text_content"`
	Label        string    `db:"label" json:"label"`
	Score        float64   `db:"score" json:"score"`
	Confidence   float64   `db:"confidence" json:"confidence"`
	ModelVersion string    `db:"model_version" json:"model_version"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
}

type ExportData struct {
	UserEmail  string           `json:"user_email"`
	UserSince  time.Time        `json:"user_since"`
	Stats      UserStats        `json:"stats"`
	Trend      DashboardTrend   `json:"trend"`
	Analyses   []ExportAnalysis `json:"analyses"`
	ExportedAt time.Time        `json:"exported_at"`
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

type TrendPoint struct {
	Period      time.Time `db:"period" json:"period"`
	AvgScore    float64   `db:"avg_score" json:"avg_score"`
	Total       int       `db:"total" json:"total"`
	HighCount   int       `db:"high_count" json:"high_count"`
	MediumCount int       `db:"medium_count" json:"medium_count"`
	LowCount    int       `db:"low_count" json:"low_count"`
}

func (s *DashboardService) TrendPoints(ctx context.Context, userID *uuid.UUID, period string, n int) ([]TrendPoint, error) {
	if period != "month" {
		period = "week"
	}
	if n <= 0 || n > 52 {
		n = 12
	}

	interval := fmt.Sprintf("%d %ss", n, period)

	var rows []TrendPoint
	var err error
	if userID != nil {
		err = s.db.SelectContext(ctx, &rows, `
			SELECT
				date_trunc($1, ar.created_at)                          AS period,
				COALESCE(ROUND(AVG(ar.score)::numeric, 4), 0)::float8  AS avg_score,
				COUNT(*)                                                AS total,
				COUNT(*) FILTER (WHERE ar.label = 'high')              AS high_count,
				COUNT(*) FILTER (WHERE ar.label = 'medium')            AS medium_count,
				COUNT(*) FILTER (WHERE ar.label = 'low')               AS low_count
			FROM analysis_results ar
			JOIN analyses a ON a.id = ar.analysis_id
			WHERE a.user_id = $2
			  AND ar.created_at >= now() - $3::interval
			GROUP BY 1
			ORDER BY 1 ASC
		`, period, *userID, interval)
	} else {
		err = s.db.SelectContext(ctx, &rows, `
			SELECT
				date_trunc($1, ar.created_at)                          AS period,
				COALESCE(ROUND(AVG(ar.score)::numeric, 4), 0)::float8  AS avg_score,
				COUNT(*)                                                AS total,
				COUNT(*) FILTER (WHERE ar.label = 'high')              AS high_count,
				COUNT(*) FILTER (WHERE ar.label = 'medium')            AS medium_count,
				COUNT(*) FILTER (WHERE ar.label = 'low')               AS low_count
			FROM analysis_results ar
			JOIN analyses a ON a.id = ar.analysis_id
			WHERE ar.created_at >= now() - $2::interval
			GROUP BY 1
			ORDER BY 1 ASC
		`, period, interval)
	}
	if err != nil {
		return nil, fmt.Errorf("trend points: %w", err)
	}
	if rows == nil {
		rows = []TrendPoint{}
	}
	return rows, nil
}

func (s *DashboardService) ExportData(ctx context.Context, userID uuid.UUID) (ExportData, error) {
	var out ExportData
	out.ExportedAt = time.Now().UTC()

	var user struct {
		Email     string    `db:"email"`
		CreatedAt time.Time `db:"created_at"`
	}
	if err := s.db.GetContext(ctx, &user, `SELECT email, created_at FROM users WHERE id = $1`, userID); err != nil {
		return ExportData{}, fmt.Errorf("get user: %w", err)
	}
	out.UserEmail = user.Email
	out.UserSince = user.CreatedAt

	stats, err := s.Stats(ctx, userID)
	if err != nil {
		return ExportData{}, err
	}
	out.Stats = stats

	summary, err := s.Summary(ctx, userID)
	if err != nil {
		return ExportData{}, err
	}
	out.Trend = summary.Trend

	if err := s.db.SelectContext(ctx, &out.Analyses, `
		SELECT
			a.id::text,
			t.content   AS text_content,
			COALESCE(ar.label, '')       AS label,
			COALESCE(ar.score, 0)        AS score,
			COALESCE(ar.confidence, 0)   AS confidence,
			a.model_version,
			a.created_at
		FROM analyses a
		JOIN texts t ON t.id = a.text_id
		LEFT JOIN analysis_results ar ON ar.analysis_id = a.id
		WHERE a.user_id = $1
		ORDER BY a.created_at DESC
	`, userID); err != nil {
		return ExportData{}, fmt.Errorf("export analyses: %w", err)
	}
	if out.Analyses == nil {
		out.Analyses = []ExportAnalysis{}
	}
	return out, nil
}

func (s *DashboardService) Stats(ctx context.Context, userID uuid.UUID) (UserStats, error) {
	type row struct {
		Label string `db:"label"`
		Cnt   int    `db:"cnt"`
	}
	var rows []row
	if err := s.db.SelectContext(ctx, &rows, `
		SELECT COALESCE(ar.label, 'unknown') AS label, COUNT(*) AS cnt
		FROM analyses a
		LEFT JOIN analysis_results ar ON ar.analysis_id = a.id
		WHERE a.user_id = $1
		GROUP BY label
	`, userID); err != nil {
		return UserStats{}, fmt.Errorf("stats by label: %w", err)
	}

	var stats UserStats
	for _, r := range rows {
		stats.Total += r.Cnt
		switch r.Label {
		case "high":
			stats.HighRisk = r.Cnt
		case "medium":
			stats.MediumRisk = r.Cnt
		case "low":
			stats.LowRisk = r.Cnt
		}
	}

	var avgConf sql.NullFloat64
	if err := s.db.GetContext(ctx, &avgConf, `
		SELECT AVG(ar.confidence)
		FROM analysis_results ar
		JOIN analyses a ON a.id = ar.analysis_id
		WHERE a.user_id = $1
	`, userID); err != nil {
		return UserStats{}, fmt.Errorf("avg confidence: %w", err)
	}
	if avgConf.Valid {
		stats.AvgConf = avgConf.Float64
	}

	if err := s.db.GetContext(ctx, &stats.ThisWeek, `
		SELECT COUNT(*)
		FROM analyses a
		WHERE a.user_id = $1 AND a.created_at >= now() - interval '7 days'
	`, userID); err != nil {
		return UserStats{}, fmt.Errorf("this week count: %w", err)
	}

	return stats, nil
}
