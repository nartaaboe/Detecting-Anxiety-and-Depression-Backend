package httpapi

import (
	"net/http"
	"strings"
	"time"

	"log/slog"

	"github.com/google/uuid"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/services"
)

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimSpace(r.Header.Get("X-Request-Id"))
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(withRequestID(r.Context(), id)))
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w}
			start := time.Now()
			next.ServeHTTP(sw, r)

			if logger == nil {
				return
			}

			fields := []any{
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", sw.status),
				slog.Int("bytes", sw.bytes),
				slog.Duration("duration", time.Since(start)),
			}

			if rid, ok := RequestIDFromContext(r.Context()); ok {
				fields = append(fields, slog.String("request_id", rid))
			}
			if a, ok := AuthFromContext(r.Context()); ok {
				fields = append(fields, slog.String("user_id", a.UserID.String()))
			}

			logger.Info("request", fields...)
		})
	}
}

func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowAll := false
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		o = strings.TrimSpace(o)
		if o == "" {
			continue
		}
		if o == "*" {
			allowAll = true
			continue
		}
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := strings.TrimSpace(r.Header.Get("Origin"))
			if origin != "" {
				if allowAll {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else if _, ok := allowed[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
				w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization,Content-Type,X-Request-Id")
				w.Header().Set("Access-Control-Expose-Headers", "X-Request-Id")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func AuthRequired(jwtm *services.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
				return
			}
			tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
			claims, err := jwtm.ParseAccess(tokenString)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token")
				return
			}

			userID, err := uuid.Parse(claims.UserID)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid access token")
				return
			}

			a := AuthInfo{UserID: userID, Roles: claims.Roles}
			next.ServeHTTP(w, r.WithContext(withAuth(r.Context(), a)))
		})
	}
}

func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, ok := AuthFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
			return
		}
		for _, role := range a.Roles {
			if role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, "forbidden", "admin role required")
	})
}
