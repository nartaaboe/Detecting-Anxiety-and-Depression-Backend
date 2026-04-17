package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/ai"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/config"
	httpapi "github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/http"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/migrations"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/repositories"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/services"
	"github.com/nartaaboe/Detecting-Anxiety-and-Depression-Backend/internal/workers"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config load failed", slog.Any("err", err))
		os.Exit(1)
	}

	appCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := repositories.Connect(appCtx, cfg.DBDSN)
	if err != nil {
		logger.Error("db connect failed", slog.Any("err", err))
		os.Exit(1)
	}
	defer db.Close()

	if err := migrations.Up(appCtx, db.DB, logger); err != nil {
		logger.Error("migrations failed", slog.Any("err", err))
		os.Exit(1)
	}
	if err := db.DB.PingContext(appCtx); err != nil {
		logger.Error("db ping after migrations failed", slog.Any("err", err))
		os.Exit(1)
	}
	// Repos
	usersRepo := repositories.NewUsersRepo(db)
	rolesRepo := repositories.NewRolesRepo(db)
	sessionsRepo := repositories.NewSessionsRepo(db)
	textsRepo := repositories.NewTextsRepo(db)
	analysesRepo := repositories.NewAnalysesRepo(db)
	resultsRepo := repositories.NewResultsRepo(db)
	auditRepo := repositories.NewAuditRepo(db)
	settingsRepo := repositories.NewSettingsRepo(db)

	if err := settingsRepo.EnsureModelSettings(appCtx, cfg.DefaultModelVersion, cfg.DefaultThreshold); err != nil {
		logger.Error("ensure model settings failed", slog.Any("err", err))
		os.Exit(1)
	}

	// Core services
	jwtm := services.NewJWTManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.AccessTTL, cfg.RefreshTTL)
	authSvc := services.NewAuthService(db, usersRepo, rolesRepo, sessionsRepo, auditRepo, jwtm)
	textSvc := services.NewTextService(db, textsRepo)

	aiClient := ai.NewClient(cfg.AIBaseURL, cfg.AITimeout)

	workerPool := workers.NewPool(cfg.WorkersCount, 1024, analysesRepo, resultsRepo, aiClient, logger)
	workersCtx, workersCancel := context.WithCancel(context.Background())
	defer workersCancel()
	workerPool.Start(workersCtx, cfg.WorkersCount)

	analysisSvc := services.NewAnalysisService(db, textsRepo, analysesRepo, resultsRepo, settingsRepo, workerPool)
	dashboardSvc := services.NewDashboardService(db)
	adminSvc := services.NewAdminService(db, usersRepo, rolesRepo, auditRepo, settingsRepo, analysesRepo)

	validate := validator.New()

	router := httpapi.NewRouter(httpapi.Handlers{
		Auth:      authSvc,
		Texts:     textSvc,
		Analyses:  analysisSvc,
		Dashboard: dashboardSvc,
		Admin:     adminSvc,
		JWT:       jwtm,
		Validate:  validate,
		Logger:    logger,
		Config:    cfg,
	})
	handler := httpapi.CORSMiddleware(cfg.CORSOrigins)(router)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("http server started", slog.Int("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed", slog.Any("err", err))
			stop()
		}
	}()

	<-appCtx.Done()

	logger.Info("shutdown started")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	workerPool.Stop()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", slog.Any("err", err))
	}

	done := make(chan struct{})
	go func() {
		workerPool.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-shutdownCtx.Done():
		workersCancel()
	}

	logger.Info("shutdown complete")
}
