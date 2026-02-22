package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func Up(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	src, err := iofs.New(FS, ".")
	if err != nil {
		return fmt.Errorf("migrations source: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("migrations db driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return fmt.Errorf("migrations init: %w", err)
	}

	err = m.Up()
	if err == nil {
		if logger != nil {
			logger.Info("migrations applied")
		}
		return nil
	}
	if errors.Is(err, migrate.ErrNoChange) {
		if logger != nil {
			logger.Info("migrations no change")
		}
		return nil
	}

	return fmt.Errorf("migrations up: %w", err)
}
