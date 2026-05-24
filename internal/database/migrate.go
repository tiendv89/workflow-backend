package database

import (
	"context"
	"io/fs"

	"github.com/tiendv89/workflow-backend/migrations"
	pkgdb "github.com/tiendv89/workflow-backend/pkg/db"
)

// MigrationFS exposes the embedded migration SQL files.
var MigrationFS fs.FS = migrations.FS

// RunMigrations applies all pending goose migrations to the target database.
func RunMigrations(ctx context.Context, databaseURL string) error {
	return pkgdb.RunMigrations(ctx, databaseURL, migrations.FS)
}

// MigrateUpN applies up to n pending migrations. n=0 means apply all.
func MigrateUpN(ctx context.Context, databaseURL string, n int) error {
	return pkgdb.MigrateUpN(ctx, databaseURL, migrations.FS, n)
}

// MigrateDownN rolls back n migrations. n=0 is a no-op.
func MigrateDownN(ctx context.Context, databaseURL string, n int) error {
	return pkgdb.MigrateDownN(ctx, databaseURL, migrations.FS, n)
}
