package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func RunMigrations(ctx context.Context, databaseURL string, migrationsFS fs.FS) error {
	return MigrateUpN(ctx, databaseURL, migrationsFS, 0)
}

func MigrateUpN(ctx context.Context, databaseURL string, migrationsFS fs.FS, n int) error {
	db, err := openMigrationDB(databaseURL, migrationsFS)
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

func MigrateDownN(ctx context.Context, databaseURL string, migrationsFS fs.FS, n int) error {
	if n == 0 {
		return nil
	}
	db, err := openMigrationDB(databaseURL, migrationsFS)
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

func openMigrationDB(databaseURL string, migrationsFS fs.FS) (*sql.DB, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open migration db: %w", err)
	}
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}
	return db, nil
}
