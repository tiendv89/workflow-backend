package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/tiendv89/workflow-backend/migrations"
)

// MigrationFS exposes the embedded migration SQL files.
var MigrationFS fs.FS = migrations.FS

// RunMigrations applies all pending goose migrations to the target database.
func RunMigrations(ctx context.Context, databaseURL string) error {
	return MigrateUpN(ctx, databaseURL, 0)
}

// MigrateUpN applies up to n pending migrations. n=0 means apply all.
func MigrateUpN(ctx context.Context, databaseURL string, n int) error {
	db, err := openMigrationDB(databaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if n == 0 {
		if err := goose.UpContext(ctx, db, "."); err != nil {
			return fmt.Errorf("migrate up: %w", err)
		}
		return nil
	}
	for i := 0; i < n; i++ {
		if err := goose.UpByOneContext(ctx, db, "."); err != nil {
			return fmt.Errorf("migrate up step %d: %w", i+1, err)
		}
	}
	return nil
}

// MigrateDownN rolls back n migrations.
func MigrateDownN(ctx context.Context, databaseURL string, n int) error {
	db, err := openMigrationDB(databaseURL)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	for i := 0; i < n; i++ {
		if err := goose.DownContext(ctx, db, "."); err != nil {
			return fmt.Errorf("migrate down step %d: %w", i+1, err)
		}
	}
	return nil
}

func openMigrationDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open migration db: %w", err)
	}
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}
	return db, nil
}
