package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
)

type AuditRepo struct {
	db *sqlx.DB
}

func NewAuditRepo(db *sqlx.DB) *AuditRepo {
	return &AuditRepo{db: db}
}

func (r *AuditRepo) Create(ctx context.Context, tx *sqlx.Tx, actorUserID *uuid.UUID, action, entityType string, entityID uuid.UUID, metaJSON []byte, ip string) error {
	if metaJSON == nil {
		metaJSON = []byte(`{}`)
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO audit_logs (actor_user_id, action, entity_type, entity_id, meta_json, ip)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, actorUserID, action, entityType, entityID, metaJSON, ip)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (r *AuditRepo) List(ctx context.Context, limit, offset int) ([]models.AuditLog, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM audit_logs`); err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	var logs []models.AuditLog
	if err := r.db.SelectContext(ctx, &logs, `
		SELECT al.id, al.actor_user_id, u.email AS actor_email,
		       al.action, al.entity_type, al.entity_id, al.meta_json, al.created_at, al.ip
		FROM audit_logs al
		LEFT JOIN users u ON u.id = al.actor_user_id
		ORDER BY al.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("list audit logs: %w", err)
	}

	return logs, total, nil
}
