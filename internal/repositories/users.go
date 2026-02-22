package repositories

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
)

type UsersRepo struct {
	db *sqlx.DB
}

func NewUsersRepo(db *sqlx.DB) *UsersRepo {
	return &UsersRepo{db: db}
}

func (r *UsersRepo) Create(ctx context.Context, tx *sqlx.Tx, email, passwordHash string, isActive bool) (models.User, error) {
	var u models.User
	if err := tx.GetContext(ctx, &u, `
		INSERT INTO users (email, password_hash, is_active)
		VALUES ($1, $2, $3)
		RETURNING id, email, password_hash, is_active, created_at
	`, email, passwordHash, isActive); err != nil {
		return models.User{}, fmt.Errorf("insert user: %w", err)
	}
	return u, nil
}

func (r *UsersRepo) GetByEmail(ctx context.Context, email string) (models.User, error) {
	var u models.User
	if err := r.db.GetContext(ctx, &u, `
		SELECT id, email, password_hash, is_active, created_at
		FROM users
		WHERE email = $1
	`, email); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *UsersRepo) GetByID(ctx context.Context, id uuid.UUID) (models.User, error) {
	var u models.User
	if err := r.db.GetContext(ctx, &u, `
		SELECT id, email, password_hash, is_active, created_at
		FROM users
		WHERE id = $1
	`, id); err != nil {
		return models.User{}, err
	}
	return u, nil
}

func (r *UsersRepo) List(ctx context.Context, limit, offset int) ([]models.User, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM users`); err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	var users []models.User
	if err := r.db.SelectContext(ctx, &users, `
		SELECT id, email, is_active, created_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}

func (r *UsersRepo) SetActive(ctx context.Context, id uuid.UUID, isActive bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET is_active = $2 WHERE id = $1`, id, isActive)
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	return nil
}

func (r *UsersRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}
