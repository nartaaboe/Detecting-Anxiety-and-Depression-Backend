package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	PasswordHash string    `db:"password_hash" json:"-"`
	IsActive     bool      `db:"is_active" json:"is_active"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`

	Roles []string `db:"-" json:"roles,omitempty"`
}

type Role struct {
	ID   int    `db:"id" json:"id"`
	Name string `db:"name" json:"name"`
}

type UserSession struct {
	ID               uuid.UUID  `db:"id" json:"id"`
	UserID           uuid.UUID  `db:"user_id" json:"user_id"`
	RefreshTokenHash string     `db:"refresh_token_hash" json:"-"`
	ExpiresAt        time.Time  `db:"expires_at" json:"expires_at"`
	CreatedAt        time.Time  `db:"created_at" json:"created_at"`
	RevokedAt        *time.Time `db:"revoked_at" json:"revoked_at,omitempty"`
}

type Text struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type AnalysisStatus string

const (
	AnalysisQueued  AnalysisStatus = "queued"
	AnalysisRunning AnalysisStatus = "running"
	AnalysisDone    AnalysisStatus = "done"
	AnalysisFailed  AnalysisStatus = "failed"
)

type Analysis struct {
	ID           uuid.UUID      `db:"id" json:"id"`
	UserID       uuid.UUID      `db:"user_id" json:"user_id"`
	TextID       uuid.UUID      `db:"text_id" json:"text_id"`
	TextContent  string         `db:"text_content" json:"text_content"`
	Status       AnalysisStatus `db:"status" json:"status"`
	ModelVersion string         `db:"model_version" json:"model_version"`
	Threshold    float64        `db:"threshold" json:"threshold"`

	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	StartedAt    *time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt   *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	ErrorMessage *string    `db:"error_message" json:"error_message,omitempty"`
}

type AnalysisListItem struct {
	ID           uuid.UUID      `db:"id" json:"id"`
	UserID       uuid.UUID      `db:"user_id" json:"user_id"`
	TextID       uuid.UUID      `db:"text_id" json:"text_id"`
	TextContent  string         `db:"text_content" json:"text_content"`
	Status       AnalysisStatus `db:"status" json:"status"`
	ModelVersion string         `db:"model_version" json:"model_version"`
	Threshold    float64        `db:"threshold" json:"threshold"`

	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	StartedAt    *time.Time `db:"started_at" json:"started_at,omitempty"`
	FinishedAt   *time.Time `db:"finished_at" json:"finished_at,omitempty"`
	ErrorMessage *string    `db:"error_message" json:"error_message,omitempty"`

	ResultLabel      *string  `db:"result_label" json:"result_label,omitempty"`
	ResultScore      *float64 `db:"result_score" json:"result_score,omitempty"`
	ResultConfidence *float64 `db:"result_confidence" json:"result_confidence,omitempty"`
}

type AdminAnalysisListItem struct {
	AnalysisListItem
	UserEmail string `db:"user_email" json:"user_email"`
}

type AnalysisResult struct {
	ID          uuid.UUID       `db:"id" json:"id"`
	AnalysisID  uuid.UUID       `db:"analysis_id" json:"analysis_id"`
	Label       string          `db:"label" json:"label"`
	Score       float64         `db:"score" json:"score"`
	Confidence  float64         `db:"confidence" json:"confidence"`
	Explanation json.RawMessage `db:"explanation_json" json:"explanation"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
}

type AuditLog struct {
	ID          uuid.UUID       `db:"id" json:"id"`
	ActorUserID *uuid.UUID      `db:"actor_user_id" json:"actor_user_id,omitempty"`
	ActorEmail  *string         `db:"actor_email" json:"actor_email,omitempty"`
	Action      string          `db:"action" json:"action"`
	EntityType  string          `db:"entity_type" json:"entity_type"`
	EntityID    uuid.UUID       `db:"entity_id" json:"entity_id"`
	Meta        json.RawMessage `db:"meta_json" json:"meta"`
	CreatedAt   time.Time       `db:"created_at" json:"created_at"`
	IP          string          `db:"ip" json:"ip"`
}

type ModelSettings struct {
	ID                  int16     `db:"id" json:"id"`
	DefaultModelVersion string    `db:"default_model_version" json:"default_model_version"`
	DefaultThreshold    float64   `db:"default_threshold" json:"default_threshold"`
	UpdatedAt           time.Time `db:"updated_at" json:"updated_at"`
}
