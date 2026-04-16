package httpapi

import (
	"net/http"
	"strings"
	"time"
)

const refreshTokenCookieName = "refresh_token"

func (h Handlers) setRefreshTokenCookie(w http.ResponseWriter, refreshToken string) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return
	}

	// Best-effort expiration alignment with token exp.
	exp := time.Now().UTC().Add(h.Config.RefreshTTL)
	if h.JWT != nil {
		if claims, err := h.JWT.ParseRefresh(refreshToken); err == nil && claims.ExpiresAt != nil {
			exp = claims.ExpiresAt.Time.UTC()
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    refreshToken,
		Path:     "/",
		Expires:  exp,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		// Local dev runs over http://, so Secure must be false here.
		Secure: false,
	})
}

func (h Handlers) clearRefreshTokenCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   false,
	})
}

func refreshTokenFromCookie(r *http.Request) string {
	c, err := r.Cookie(refreshTokenCookieName)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}
