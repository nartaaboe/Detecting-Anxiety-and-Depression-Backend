package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/models"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/repositories"
	"golang.org/x/crypto/bcrypt"
)

type AdminStats struct {
	UsersTotal  int `json:"users_total"`
	UsersActive int `json:"users_active"`

	AnalysesTotal   int `json:"analyses_total"`
	AnalysesQueued  int `json:"analyses_queued"`
	AnalysesRunning int `json:"analyses_running"`
	AnalysesDone    int `json:"analyses_done"`
	AnalysesFailed  int `json:"analyses_failed"`
}

type AdminService struct {
	db       *sqlx.DB
	users    *repositories.UsersRepo
	roles    *repositories.RolesRepo
	audit    *repositories.AuditRepo
	settings *repositories.SettingsRepo
	analyses *repositories.AnalysesRepo
}

func NewAdminService(
	db *sqlx.DB,
	users *repositories.UsersRepo,
	roles *repositories.RolesRepo,
	audit *repositories.AuditRepo,
	settings *repositories.SettingsRepo,
	analyses *repositories.AnalysesRepo,
) *AdminService {
	return &AdminService{
		db:       db,
		users:    users,
		roles:    roles,
		audit:    audit,
		settings: settings,
		analyses: analyses,
	}
}

func (s *AdminService) ListUsers(ctx context.Context, limit, offset int) ([]models.User, int, error) {
	users, total, err := s.users.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	for i := range users {
		roles, err := s.roles.GetUserRoles(ctx, users[i].ID)
		if err != nil {
			return nil, 0, err
		}
		users[i].Roles = roles
	}
	return users, total, nil
}

type AdminCreateUserInput struct {
	Email    string
	Password string
	Role     string
	IsActive *bool
}

func (s *AdminService) CreateUser(ctx context.Context, actorUserID uuid.UUID, actorIP string, in AdminCreateUserInput) (models.User, error) {
	email := strings.ToLower(strings.TrimSpace(in.Email))
	password := strings.TrimSpace(in.Password)
	role := strings.TrimSpace(in.Role)
	if role == "" {
		role = "user"
	}
	if role != "user" && role != "admin" {
		return models.User{}, fmt.Errorf("%w: role must be 'user' or 'admin'", ErrBadRequest)
	}

	if email == "" || password == "" {
		return models.User{}, fmt.Errorf("%w: email and password are required", ErrBadRequest)
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	pwHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return models.User{}, fmt.Errorf("hash password: %w", err)
	}

	var created models.User
	err = repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		u, err := s.users.Create(ctx, tx, email, string(pwHash), isActive)
		if err != nil {
			if repositories.IsUniqueViolation(err) {
				return fmt.Errorf("%w: email already exists", ErrConflict)
			}
			return err
		}

		if err := s.roles.SetUserRole(ctx, tx, u.ID, role); err != nil {
			return err
		}

		roles, err := s.roles.GetUserRolesTx(ctx, tx, u.ID)
		if err != nil {
			return err
		}
		u.Roles = roles

		meta, _ := json.Marshal(map[string]any{"email": u.Email, "role": role, "is_active": u.IsActive})
		if err := s.audit.Create(ctx, tx, &actorUserID, "admin_create_user", "user", u.ID, meta, actorIP); err != nil {
			return err
		}

		created = u
		return nil
	})
	if err != nil {
		return models.User{}, err
	}

	return created, nil
}

func (s *AdminService) SetUserRole(ctx context.Context, actorUserID uuid.UUID, actorIP string, userID uuid.UUID, role string) error {
	role = strings.TrimSpace(role)
	if role != "user" && role != "admin" {
		return fmt.Errorf("%w: role must be 'user' or 'admin'", ErrBadRequest)
	}

	return repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		var exists bool
		if err := tx.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		if err := s.roles.SetUserRole(ctx, tx, userID, role); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("%w: role not found", ErrBadRequest)
			}
			return err
		}

		meta, _ := json.Marshal(map[string]any{"role": role})
		return s.audit.Create(ctx, tx, &actorUserID, "admin_set_user_role", "user", userID, meta, actorIP)
	})
}

func (s *AdminService) SetUserStatus(ctx context.Context, actorUserID uuid.UUID, actorIP string, userID uuid.UUID, isActive bool) error {
	return repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `UPDATE users SET is_active = $2 WHERE id = $1`, userID, isActive)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		meta, _ := json.Marshal(map[string]any{"is_active": isActive})
		return s.audit.Create(ctx, tx, &actorUserID, "admin_set_user_status", "user", userID, meta, actorIP)
	})
}

func (s *AdminService) DeleteUser(ctx context.Context, actorUserID uuid.UUID, actorIP string, userID uuid.UUID) error {
	return repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		res, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, userID)
		if err != nil {
			return err
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}

		meta, _ := json.Marshal(map[string]any{})
		return s.audit.Create(ctx, tx, &actorUserID, "admin_delete_user", "user", userID, meta, actorIP)
	})
}

func (s *AdminService) ListAnalyses(ctx context.Context, f repositories.AnalysisListFilter) ([]models.AdminAnalysisListItem, int, error) {
	return s.analyses.ListAll(ctx, f)
}

func (s *AdminService) ListAuditLogs(ctx context.Context, limit, offset int) ([]models.AuditLog, int, error) {
	return s.audit.List(ctx, limit, offset)
}

func (s *AdminService) Stats(ctx context.Context) (AdminStats, error) {
	var out AdminStats

	if err := s.db.GetContext(ctx, &out.UsersTotal, `SELECT COUNT(*) FROM users`); err != nil {
		return AdminStats{}, err
	}
	if err := s.db.GetContext(ctx, &out.UsersActive, `SELECT COUNT(*) FROM users WHERE is_active = true`); err != nil {
		return AdminStats{}, err
	}
	if err := s.db.GetContext(ctx, &out.AnalysesTotal, `SELECT COUNT(*) FROM analyses`); err != nil {
		return AdminStats{}, err
	}

	type row struct {
		Status string `db:"status"`
		Cnt    int    `db:"cnt"`
	}
	var rows []row
	if err := s.db.SelectContext(ctx, &rows, `SELECT status, COUNT(*) AS cnt FROM analyses GROUP BY status`); err != nil {
		return AdminStats{}, err
	}
	for _, r := range rows {
		switch r.Status {
		case "queued":
			out.AnalysesQueued = r.Cnt
		case "running":
			out.AnalysesRunning = r.Cnt
		case "done":
			out.AnalysesDone = r.Cnt
		case "failed":
			out.AnalysesFailed = r.Cnt
		}
	}

	return out, nil
}

func (s *AdminService) UpdateModelSettings(ctx context.Context, actorUserID uuid.UUID, actorIP string, defaultModelVersion string, defaultThreshold float64) (models.ModelSettings, error) {
	defaultModelVersion = strings.TrimSpace(defaultModelVersion)
	if defaultModelVersion == "" {
		return models.ModelSettings{}, fmt.Errorf("%w: default_model_version is required", ErrBadRequest)
	}
	if defaultThreshold <= 0 || defaultThreshold > 1 {
		return models.ModelSettings{}, fmt.Errorf("%w: default_threshold must be in (0,1]", ErrBadRequest)
	}

	var out models.ModelSettings
	err := repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		saved, err := s.settings.UpdateModelSettings(ctx, tx, defaultModelVersion, defaultThreshold)
		if err != nil {
			return err
		}

		meta, _ := json.Marshal(map[string]any{"default_model_version": saved.DefaultModelVersion, "default_threshold": saved.DefaultThreshold})
		if err := s.audit.Create(ctx, tx, &actorUserID, "admin_update_model_settings", "model_settings", uuid.Nil, meta, actorIP); err != nil {
			return err
		}

		out = saved
		return nil
	})
	if err != nil {
		return models.ModelSettings{}, err
	}
	return out, nil
}
