package postgres

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"

	"game-wallet-ledger/pkg/logger"
)

// RunMigrations executes all .up.sql files located in migrationsDir against the database.
func (db *DB) RunMigrations(ctx context.Context, migrationsDir string) error {
	log := logger.FromContext(ctx)

	// Ensure schema_migrations table exists to track executed migrations
	initQuery := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version VARCHAR(128) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.Pool.Exec(ctx, initQuery); err != nil {
		return fmt.Errorf("failed to initialize schema_migrations table: %w", err)
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		log.Warn("could not read migrations directory", "dir", migrationsDir, "error", err.Error())
		return nil
	}

	var upFiles []string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".up.sql") {
			upFiles = append(upFiles, f.Name())
		}
	}
	sort.Strings(upFiles)

	for _, filename := range upFiles {
		var exists int
		checkQuery := `SELECT 1 FROM schema_migrations WHERE version = $1`
		err := db.Pool.QueryRow(ctx, checkQuery, filename).Scan(&exists)
		if err == nil {
			// Already applied
			continue
		}

		filePath := filepath.Join(migrationsDir, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", filePath, err)
		}

		log.Info("applying database migration", "file", filename)
		err = db.WithinTransaction(ctx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, string(content)); err != nil {
				return fmt.Errorf("failed to execute migration %s: %w", filename, err)
			}
			if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, filename); err != nil {
				return fmt.Errorf("failed to record migration %s: %w", filename, err)
			}
			return nil
		})
		if err != nil {
			return err
		}
		log.Info("successfully applied migration", "file", filename)
	}

	return nil
}
