package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/repositories"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/services"
)

type authUserDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type authSessionDTO struct {
	AccessToken  string      `json:"accessToken"`
	RefreshToken string      `json:"refreshToken"`
	User         authUserDTO `json:"user"`
}

type meUserDTO struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	CreatedAt string `json:"createdAt,omitempty"`
}

func roleFromRoles(roles []string) string {
	for _, r := range roles {
		if r == "admin" {
			return "admin"
		}
	}
	return "user"
}

func (h Handlers) handleHealth() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeData(w, http.StatusOK, map[string]any{"status": "ok"})
	}
}

func (h Handlers) handleRegister() http.HandlerFunc {
	type req struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if err := h.Validate.Struct(body); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		u, tokens, err := h.Auth.Register(r.Context(), body.Email, body.Password)
		if err != nil {
			writeAppError(w, err)
			return
		}

		h.setRefreshTokenCookie(w, tokens.RefreshToken)
		writeData(w, http.StatusCreated, authSessionDTO{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			User: authUserDTO{
				ID:    u.ID.String(),
				Email: u.Email,
				Role:  roleFromRoles(u.Roles),
			},
		})
	}
}

func (h Handlers) handleLogin() http.HandlerFunc {
	type req struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if err := h.Validate.Struct(body); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		u, tokens, err := h.Auth.Login(r.Context(), body.Email, body.Password, clientIP(r))
		if err != nil {
			writeAppError(w, err)
			return
		}

		h.setRefreshTokenCookie(w, tokens.RefreshToken)
		writeData(w, http.StatusOK, authSessionDTO{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			User: authUserDTO{
				ID:    u.ID.String(),
				Email: u.Email,
				Role:  roleFromRoles(u.Roles),
			},
		})
	}
}

func (h Handlers) handleRefresh() http.HandlerFunc {
	type req struct {
		RefreshToken      string `json:"refresh_token"`
		RefreshTokenCamel string `json:"refreshToken"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}

		refreshToken := strings.TrimSpace(body.RefreshToken)
		if refreshToken == "" {
			refreshToken = strings.TrimSpace(body.RefreshTokenCamel)
		}
		if refreshToken == "" {
			refreshToken = refreshTokenFromCookie(r)
		}
		if refreshToken == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "refresh_token is required")
			return
		}

		tokens, err := h.Auth.Refresh(r.Context(), refreshToken)
		if err != nil {
			writeAppError(w, err)
			return
		}

		h.setRefreshTokenCookie(w, tokens.RefreshToken)
		writeData(w, http.StatusOK, tokens)
	}
}

func (h Handlers) handleLogout() http.HandlerFunc {
	type req struct {
		RefreshToken      string `json:"refresh_token"`
		RefreshTokenCamel string `json:"refreshToken"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}

		refreshToken := strings.TrimSpace(body.RefreshToken)
		if refreshToken == "" {
			refreshToken = strings.TrimSpace(body.RefreshTokenCamel)
		}
		if refreshToken == "" {
			refreshToken = refreshTokenFromCookie(r)
		}
		if refreshToken != "" {
			if err := h.Auth.Logout(r.Context(), refreshToken, clientIP(r)); err != nil && !errors.Is(err, services.ErrUnauthorized) {
				writeAppError(w, err)
				return
			}
		}

		h.clearRefreshTokenCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) handleLogoutAll() http.HandlerFunc {
	type req struct {
		RefreshToken      string `json:"refresh_token"`
		RefreshTokenCamel string `json:"refreshToken"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}

		refreshToken := strings.TrimSpace(body.RefreshToken)
		if refreshToken == "" {
			refreshToken = strings.TrimSpace(body.RefreshTokenCamel)
		}
		if refreshToken == "" {
			refreshToken = refreshTokenFromCookie(r)
		}
		if refreshToken != "" {
			if err := h.Auth.LogoutAll(r.Context(), refreshToken); err != nil && !errors.Is(err, services.ErrUnauthorized) {
				writeAppError(w, err)
				return
			}
		}

		h.clearRefreshTokenCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) handleMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		u, err := h.Auth.Me(r.Context(), a.UserID)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, meUserDTO{
			ID:        u.ID.String(),
			Email:     u.Email,
			Role:      roleFromRoles(u.Roles),
			CreatedAt: u.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
}

func (h Handlers) handleChangePassword() http.HandlerFunc {
	type req struct {
		CurrentPassword      string `json:"currentPassword"`
		NewPassword          string `json:"newPassword"`
		CurrentPasswordSnake string `json:"current_password"`
		NewPasswordSnake     string `json:"new_password"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}

		current := strings.TrimSpace(body.CurrentPassword)
		if current == "" {
			current = strings.TrimSpace(body.CurrentPasswordSnake)
		}
		next := strings.TrimSpace(body.NewPassword)
		if next == "" {
			next = strings.TrimSpace(body.NewPasswordSnake)
		}
		if current == "" || next == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "currentPassword and newPassword are required")
			return
		}

		if err := h.Auth.ChangePassword(r.Context(), a.UserID, current, next); err != nil {
			writeAppError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) handleDeleteMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		if err := h.Auth.DeleteAccount(r.Context(), a.UserID); err != nil {
			writeAppError(w, err)
			return
		}

		h.clearRefreshTokenCookie(w)
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) handleCreateText() http.HandlerFunc {
	type req struct {
		Content string `json:"content" validate:"required,min=1"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if err := h.Validate.Struct(body); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		t, err := h.Texts.Create(r.Context(), a.UserID, body.Content)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusCreated, map[string]any{"text": t})
	}
}

func (h Handlers) handleListTexts() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		limit, offset := parseLimitOffset(r)

		items, total, err := h.Texts.List(r.Context(), a.UserID, limit, offset)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{
			"items":  items,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func (h Handlers) handleGetText() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		t, err := h.Texts.Get(r.Context(), a.UserID, id)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{"text": t})
	}
}

func (h Handlers) handleUpdateText() http.HandlerFunc {
	type req struct {
		Content string `json:"content" validate:"required,min=1"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if err := h.Validate.Struct(body); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		t, err := h.Texts.Update(r.Context(), a.UserID, id, body.Content)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{"text": t})
	}
}

func (h Handlers) handleDeleteText() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		if err := h.Texts.Delete(r.Context(), a.UserID, id); err != nil {
			writeAppError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) handleCreateAnalysis() http.HandlerFunc {
	type explanation struct {
		KeyPhrases   []string `json:"key_phrases"`
		TopSentences []string `json:"top_sentences"`
	}
	type result struct {
		Label       string      `json:"label"`
		Score       float64     `json:"score"`
		Confidence  float64     `json:"confidence"`
		Explanation explanation `json:"explanation"`
	}
	type req struct {
		TextID       string   `json:"text_id"`
		Content      string   `json:"content"`
		ModelVersion string   `json:"model_version"`
		Threshold    *float64 `json:"threshold"`
		Result       *result  `json:"result"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}

		var textID *uuid.UUID
		if strings.TrimSpace(body.TextID) != "" {
			id, err := uuid.Parse(strings.TrimSpace(body.TextID))
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "text_id must be a valid uuid")
				return
			}
			textID = &id
		}

		var content *string
		if strings.TrimSpace(body.Content) != "" {
			c := body.Content
			content = &c
		}

		if body.Result != nil {
			explJSON, err := json.Marshal(body.Result.Explanation)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_error", "invalid result explanation")
				return
			}

			analysis, err := h.Analyses.CreateWithResult(r.Context(), a.UserID, services.CreateAnalysisInput{
				TextID:       textID,
				Content:      content,
				ModelVersion: body.ModelVersion,
				Threshold:    body.Threshold,
			}, services.CreateAnalysisResultInput{
				Label:           body.Result.Label,
				Score:           body.Result.Score,
				Confidence:      body.Result.Confidence,
				ExplanationJSON: explJSON,
			})
			if err != nil {
				writeAppError(w, err)
				return
			}

			writeData(w, http.StatusCreated, map[string]any{"analysis": analysis})
			return
		}

		analysis, err := h.Analyses.Create(r.Context(), a.UserID, services.CreateAnalysisInput{
			TextID:       textID,
			Content:      content,
			ModelVersion: body.ModelVersion,
			Threshold:    body.Threshold,
		})
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusCreated, map[string]any{"analysis": analysis})
	}
}

func (h Handlers) handleListAnalyses() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		limit, offset := parseLimitOffset(r)

		q := r.URL.Query()
		from, err := parseRFC3339Ptr(q.Get("from"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		to, err := parseRFC3339Ptr(q.Get("to"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		items, total, err := h.Analyses.List(r.Context(), a.UserID, repositories.AnalysisListFilter{
			Status: strings.TrimSpace(q.Get("status")),
			Label:  strings.TrimSpace(q.Get("label")),
			From:   from,
			To:     to,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{
			"items":  items,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func (h Handlers) handleGetAnalysis() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		analysis, err := h.Analyses.Get(r.Context(), a.UserID, id)
		if err != nil {
			writeAppError(w, err)
			return
		}
		writeData(w, http.StatusOK, map[string]any{"analysis": analysis})
	}
}

func (h Handlers) handleUpdateAnalysis() http.HandlerFunc {
	type explanation struct {
		KeyPhrases   []string `json:"key_phrases"`
		TopSentences []string `json:"top_sentences"`
	}
	type result struct {
		Label       string      `json:"label"`
		Score       float64     `json:"score"`
		Confidence  float64     `json:"confidence"`
		Explanation explanation `json:"explanation"`
	}
	type req struct {
		Result *result `json:"result"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if body.Result == nil {
			writeError(w, http.StatusBadRequest, "validation_error", "result is required")
			return
		}

		explJSON, err := json.Marshal(body.Result.Explanation)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "invalid result explanation")
			return
		}

		updated, err := h.Analyses.UpsertResult(r.Context(), a.UserID, id, services.CreateAnalysisResultInput{
			Label:           body.Result.Label,
			Score:           body.Result.Score,
			Confidence:      body.Result.Confidence,
			ExplanationJSON: explJSON,
		})
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{"analysis": updated})
	}
}

func (h Handlers) handleDeleteAnalysis() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		if err := h.Analyses.Delete(r.Context(), a.UserID, id); err != nil {
			writeAppError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) handleGetAnalysisResult() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		id, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		res, err := h.Analyses.GetResult(r.Context(), a.UserID, id)
		if err != nil {
			writeAppError(w, err)
			return
		}
		writeData(w, http.StatusOK, map[string]any{"result": res})
	}
}

func (h Handlers) handleDashboardSummary() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		sum, err := h.Dashboard.Summary(r.Context(), a.UserID)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{"summary": sum})
	}
}

// --- Admin ---

func (h Handlers) handleAdminListUsers() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parseLimitOffset(r)

		users, total, err := h.Admin.ListUsers(r.Context(), limit, offset)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{
			"items":  users,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func (h Handlers) handleAdminCreateUser() http.HandlerFunc {
	type req struct {
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=8"`
		Role     string `json:"role" validate:"omitempty,oneof=user admin"`
		IsActive *bool  `json:"is_active"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if err := h.Validate.Struct(body); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		u, err := h.Admin.CreateUser(r.Context(), a.UserID, clientIP(r), services.AdminCreateUserInput{
			Email:    body.Email,
			Password: body.Password,
			Role:     body.Role,
			IsActive: body.IsActive,
		})
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusCreated, map[string]any{"user": u})
	}
}

func (h Handlers) handleAdminSetUserRole() http.HandlerFunc {
	type req struct {
		Role string `json:"role" validate:"required,oneof=user admin"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		userID, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if err := h.Validate.Struct(body); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		updated, err := h.Admin.SetUserRole(r.Context(), a.UserID, clientIP(r), userID, body.Role)
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{"user": updated})
	}
}

func (h Handlers) handleAdminSetUserStatus() http.HandlerFunc {
	type req struct {
		IsActive *bool `json:"is_active" validate:"required"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		userID, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if err := h.Validate.Struct(body); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		if err := h.Admin.SetUserStatus(r.Context(), a.UserID, clientIP(r), userID, *body.IsActive); err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{"status": "ok"})
	}
}

func (h Handlers) handleAdminDeleteUser() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		idStr := mux.Vars(r)["id"]
		userID, err := uuid.Parse(strings.TrimSpace(idStr))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", "id must be a valid uuid")
			return
		}

		if err := h.Admin.DeleteUser(r.Context(), a.UserID, clientIP(r), userID); err != nil {
			writeAppError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) handleAdminListAnalyses() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parseLimitOffset(r)
		q := r.URL.Query()

		from, err := parseRFC3339Ptr(q.Get("from"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}
		to, err := parseRFC3339Ptr(q.Get("to"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		items, total, err := h.Admin.ListAnalyses(r.Context(), repositories.AnalysisListFilter{
			Status: strings.TrimSpace(q.Get("status")),
			Label:  strings.TrimSpace(q.Get("label")),
			From:   from,
			To:     to,
			Limit:  limit,
			Offset: offset,
		})
		if err != nil {
			writeAppError(w, err)
			return
		}

		writeData(w, http.StatusOK, map[string]any{
			"items":  items,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func (h Handlers) handleAdminListAuditLogs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit, offset := parseLimitOffset(r)
		logs, total, err := h.Admin.ListAuditLogs(r.Context(), limit, offset)
		if err != nil {
			writeAppError(w, err)
			return
		}
		writeData(w, http.StatusOK, map[string]any{
			"items":  logs,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		})
	}
}

func (h Handlers) handleAdminStats() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := h.Admin.Stats(r.Context())
		if err != nil {
			writeAppError(w, err)
			return
		}
		writeData(w, http.StatusOK, map[string]any{"stats": stats})
	}
}

func (h Handlers) handleAdminUpdateModelSettings() http.HandlerFunc {
	type req struct {
		DefaultModelVersion string  `json:"default_model_version" validate:"required"`
		DefaultThreshold    float64 `json:"default_threshold" validate:"required"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}

		var body req
		if err := decodeJSON(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "invalid json body")
			return
		}
		if err := h.Validate.Struct(body); err != nil {
			writeError(w, http.StatusBadRequest, "validation_error", err.Error())
			return
		}

		saved, err := h.Admin.UpdateModelSettings(r.Context(), a.UserID, clientIP(r), body.DefaultModelVersion, body.DefaultThreshold)
		if err != nil {
			writeAppError(w, err)
			return
		}
		writeData(w, http.StatusOK, map[string]any{"model_settings": saved})
	}
}
