package repositories

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type RolesRepo struct {
	db *sqlx.DB
}

func NewRolesRepo(db *sqlx.DB) *RolesRepo {
	return &RolesRepo{db: db}
}

func (r *RolesRepo) GetRoleIDByName(ctx context.Context, name string) (int, error) {
	var id int
	if err := r.db.GetContext(ctx, &id, `SELECT id FROM roles WHERE name = $1`, name); err != nil {
		return 0, err
	}
	return id, nil
}

func (r *RolesRepo) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var roles []string
	if err := r.db.SelectContext(ctx, &roles, `
		SELECT ro.name
		FROM user_roles ur
		JOIN roles ro ON ro.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY ro.name
	`, userID); err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *RolesRepo) GetUserRolesTx(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID) ([]string, error) {
	var roles []string
	if err := tx.SelectContext(ctx, &roles, `
		SELECT ro.name
		FROM user_roles ur
		JOIN roles ro ON ro.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY ro.name
	`, userID); err != nil {
		return nil, err
	}
	return roles, nil
}

func (r *RolesRepo) SetUserRole(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, roleName string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM user_roles WHERE user_id = $1`, userID); err != nil {
		return fmt.Errorf("delete user roles: %w", err)
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO user_roles (user_id, role_id)
		SELECT $1, id FROM roles WHERE name = $2
	`, userID, roleName)
	if err != nil {
		return fmt.Errorf("insert user role: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("insert user role rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}
