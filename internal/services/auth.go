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
	"golang.org/x/crypto/bcrypt"
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthService struct {
	db       *sqlx.DB
	users    *repositories.UsersRepo
	roles    *repositories.RolesRepo
	sessions *repositories.SessionsRepo
	jwt      *JWTManager
}

func NewAuthService(db *sqlx.DB, users *repositories.UsersRepo, roles *repositories.RolesRepo, sessions *repositories.SessionsRepo, jwt *JWTManager) *AuthService {
	return &AuthService{
		db:       db,
		users:    users,
		roles:    roles,
		sessions: sessions,
		jwt:      jwt,
	}
}

func (s *AuthService) Register(ctx context.Context, email, password string) (models.User, TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return models.User{}, TokenPair{}, fmt.Errorf("%w: email and password are required", ErrBadRequest)
	}

	pwHash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return models.User{}, TokenPair{}, fmt.Errorf("hash password: %w", err)
	}

	var outUser models.User
	var tokens TokenPair

	err = repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		u, err := s.users.Create(ctx, tx, email, string(pwHash), true)
		if err != nil {
			if repositories.IsUniqueViolation(err) {
				return fmt.Errorf("%w: email already exists", ErrConflict)
			}
			return err
		}

		if err := s.roles.SetUserRole(ctx, tx, u.ID, "user"); err != nil {
			return err
		}

		roles, err := s.roles.GetUserRolesTx(ctx, tx, u.ID)
		if err != nil {
			return err
		}
		u.Roles = roles

		pair, err := s.issueTokens(ctx, tx, u.ID, roles, time.Now().UTC())
		if err != nil {
			return err
		}

		outUser = u
		tokens = pair
		return nil
	})
	if err != nil {
		return models.User{}, TokenPair{}, err
	}
	return outUser, tokens, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (models.User, TokenPair, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return models.User{}, TokenPair{}, fmt.Errorf("%w: email and password are required", ErrBadRequest)
	}

	u, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.User{}, TokenPair{}, fmt.Errorf("%w: invalid credentials", ErrUnauthorized)
		}
		return models.User{}, TokenPair{}, err
	}
	if !u.IsActive {
		return models.User{}, TokenPair{}, fmt.Errorf("%w: user is inactive", ErrForbidden)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return models.User{}, TokenPair{}, fmt.Errorf("%w: invalid credentials", ErrUnauthorized)
	}

	roles, err := s.roles.GetUserRoles(ctx, u.ID)
	if err != nil {
		return models.User{}, TokenPair{}, err
	}
	u.Roles = roles

	var tokens TokenPair
	err = repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		pair, err := s.issueTokens(ctx, tx, u.ID, roles, time.Now().UTC())
		if err != nil {
			return err
		}
		tokens = pair
		return nil
	})
	if err != nil {
		return models.User{}, TokenPair{}, err
	}

	return u, tokens, nil
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return TokenPair{}, fmt.Errorf("%w: refresh_token is required", ErrBadRequest)
	}

	claims, err := s.jwt.ParseRefresh(refreshToken)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
	}

	refreshHash := HashToken(refreshToken)

	var out TokenPair
	err = repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		var sess models.UserSession
		if err := tx.GetContext(ctx, &sess, `
			SELECT id, user_id, refresh_token_hash, expires_at, created_at, revoked_at
			FROM user_sessions
			WHERE id = $1
		`, sessionID); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
			}
			return err
		}

		if sess.UserID != userID {
			return fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
		}
		if sess.RevokedAt != nil {
			return fmt.Errorf("%w: refresh token revoked", ErrUnauthorized)
		}
		if time.Now().UTC().After(sess.ExpiresAt) {
			return fmt.Errorf("%w: refresh token expired", ErrUnauthorized)
		}
		if !TokenHashEqual(sess.RefreshTokenHash, refreshHash) {
			return fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
		}

		// Rotate: revoke old session and create a new one.
		now := time.Now().UTC()
		if err := s.sessions.Revoke(ctx, tx, sessionID, now); err != nil {
			return err
		}

		roles, err := s.roles.GetUserRoles(ctx, userID)
		if err != nil {
			return err
		}

		pair, err := s.issueTokens(ctx, tx, userID, roles, now)
		if err != nil {
			return err
		}
		out = pair
		return nil
	})
	if err != nil {
		return TokenPair{}, err
	}

	return out, nil
}

func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return fmt.Errorf("%w: refresh_token is required", ErrBadRequest)
	}

	claims, err := s.jwt.ParseRefresh(refreshToken)
	if err != nil {
		return fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
	}

	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
	}

	return repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		if err := s.sessions.Revoke(ctx, tx, sessionID, time.Now().UTC()); err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		return nil
	})
}

func (s *AuthService) LogoutAll(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return fmt.Errorf("%w: refresh_token is required", ErrBadRequest)
	}

	claims, err := s.jwt.ParseRefresh(refreshToken)
	if err != nil {
		return fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
	}

	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return fmt.Errorf("%w: invalid refresh token", ErrUnauthorized)
	}

	return repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		return s.sessions.RevokeAllForUser(ctx, tx, userID, time.Now().UTC())
	})
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	currentPassword = strings.TrimSpace(currentPassword)
	newPassword = strings.TrimSpace(newPassword)
	if currentPassword == "" || newPassword == "" {
		return fmt.Errorf("%w: current and new password are required", ErrBadRequest)
	}
	if len(newPassword) < 8 {
		return fmt.Errorf("%w: new password must be at least 8 characters", ErrBadRequest)
	}

	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: user not found", ErrNotFound)
		}
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return fmt.Errorf("%w: current password is incorrect", ErrBadRequest)
	}

	pwHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	return repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		return s.users.UpdatePasswordHash(ctx, tx, userID, string(pwHash))
	})
}

func (s *AuthService) DeleteAccount(ctx context.Context, userID uuid.UUID) error {
	return repositories.WithinTx(ctx, s.db, nil, func(tx *sqlx.Tx) error {
		return s.users.DeleteTx(ctx, tx, userID)
	})
}

func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (models.User, error) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return models.User{}, fmt.Errorf("%w: user not found", ErrNotFound)
		}
		return models.User{}, err
	}
	roles, err := s.roles.GetUserRoles(ctx, userID)
	if err != nil {
		return models.User{}, err
	}
	u.Roles = roles
	return u, nil
}

func (s *AuthService) issueTokens(ctx context.Context, tx *sqlx.Tx, userID uuid.UUID, roles []string, now time.Time) (TokenPair, error) {
	access, _, err := s.jwt.NewAccessToken(userID, roles, now)
	if err != nil {
		return TokenPair{}, err
	}

	sessionID := uuid.New()
	refresh, refreshExp, err := s.jwt.NewRefreshToken(sessionID, userID, roles, now)
	if err != nil {
		return TokenPair{}, err
	}

	refreshHash := HashToken(refresh)
	if _, err := s.sessions.Create(ctx, tx, sessionID, userID, refreshHash, refreshExp); err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}
