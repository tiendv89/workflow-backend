package db

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, config Config) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(config.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}

	if config.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(config.MaxOpenConns)
	}
	if config.MaxIdleConns > 0 {
		poolCfg.MinConns = int32(config.MaxIdleConns)
	}
	if config.ConnLifeTimeSeconds > 0 {
		poolCfg.MaxConnLifetime = time.Duration(config.ConnLifeTimeSeconds) * time.Second
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.NewWithConfig: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}
