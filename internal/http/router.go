package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/config"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/services"
)

type Handlers struct {
	Auth      *services.AuthService
	Texts     *services.TextService
	Analyses  *services.AnalysisService
	Dashboard *services.DashboardService
	Admin     *services.AdminService

	JWT *services.JWTManager

	Validate *validator.Validate
	Logger   *slog.Logger
	Config   config.Config
}

func NewRouter(h Handlers) *mux.Router {
	r := mux.NewRouter()

	r.Use(RecoveryMiddleware(h.Logger))
	r.Use(RequestIDMiddleware)
	r.Use(LoggingMiddleware(h.Logger))
	r.Use(CORSMiddleware(h.Config.CORSOrigins))

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "route not found")
	})
	r.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})

	r.HandleFunc("/health", h.handleHealth()).Methods("GET")

	r.HandleFunc("/auth/register", h.handleRegister()).Methods("POST")
	r.HandleFunc("/auth/login", h.handleLogin()).Methods("POST")
	r.HandleFunc("/auth/refresh", h.handleRefresh()).Methods("POST")
	r.HandleFunc("/auth/logout", h.handleLogout()).Methods("POST")

	protected := r.NewRoute().Subrouter()
	protected.Use(AuthRequired(h.JWT))

	protected.HandleFunc("/auth/me", h.handleMe()).Methods("GET")

	protected.HandleFunc("/texts", h.handleCreateText()).Methods("POST")

	protected.HandleFunc("/analyses", h.handleCreateAnalysis()).Methods("POST")
	protected.HandleFunc("/analyses", h.handleListAnalyses()).Methods("GET")
	protected.HandleFunc("/analyses/{id}", h.handleGetAnalysis()).Methods("GET")
	protected.HandleFunc("/analyses/{id}/result", h.handleGetAnalysisResult()).Methods("GET")

	protected.HandleFunc("/dashboard/summary", h.handleDashboardSummary()).Methods("GET")

	admin := r.PathPrefix("/admin").Subrouter()
	admin.Use(AuthRequired(h.JWT))
	admin.Use(AdminOnly)

	admin.HandleFunc("/users", h.handleAdminListUsers()).Methods("GET")
	admin.HandleFunc("/users", h.handleAdminCreateUser()).Methods("POST")
	admin.HandleFunc("/users/{id}/role", h.handleAdminSetUserRole()).Methods("PATCH")
	admin.HandleFunc("/users/{id}/status", h.handleAdminSetUserStatus()).Methods("PATCH")
	admin.HandleFunc("/users/{id}", h.handleAdminDeleteUser()).Methods("DELETE")

	admin.HandleFunc("/analyses", h.handleAdminListAnalyses()).Methods("GET")
	admin.HandleFunc("/audit-logs", h.handleAdminListAuditLogs()).Methods("GET")
	admin.HandleFunc("/stats", h.handleAdminStats()).Methods("GET")
	admin.HandleFunc("/model-settings", h.handleAdminUpdateModelSettings()).Methods("PATCH")

	return r
}
