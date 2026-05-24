package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	pkgdb "github.com/tiendv89/workflow-backend/pkg/db"
)

// Pool wraps pgxpool.Pool for the api-service read queries.
type Pool struct {
	*pgxpool.Pool
}

// Connect creates a new Pool from the given db config.
func Connect(ctx context.Context, config pkgdb.Config) (*Pool, error) {
	pool, err := pkgdb.NewPool(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("database: %w", err)
	}
	return &Pool{pool}, nil
}
