// Package migration provides the cobra subcommand for running database migrations.
package migration

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/tiendv89/workflow-backend/configs"
	"github.com/tiendv89/workflow-backend/internal/database"
)

var (
	upSteps   int
	downSteps int
)

// Command is the cobra subcommand for running database migrations.
var Command = func() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migration",
		Short: "Run database migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(upSteps, downSteps)
		},
	}
	cmd.Flags().IntVarP(&upSteps, "up", "u", 0, "apply N migrations up; 0 means all")
	cmd.Flags().IntVarP(&downSteps, "down", "d", 0, "roll back N migrations")
	return cmd
}()

func run(upSteps, downSteps int) error {
	if upSteps > 0 && downSteps > 0 {
		return fmt.Errorf("cannot specify both -u and -d")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if downSteps > 0 {
		log.Info().Int("steps", downSteps).Msg("rolling back migrations")
		if err := database.MigrateDownN(ctx, configs.G.DB.DSN(), downSteps); err != nil {
			return fmt.Errorf("migrate down: %w", err)
		}
		log.Info().Int("steps", downSteps).Msg("rollback complete")
		return nil
	}

	// upSteps == 0 means apply all
	log.Info().Int("steps", upSteps).Msg("applying migrations")
	if err := database.MigrateUpN(ctx, configs.G.DB.DSN(), upSteps); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	log.Info().Msg("migrations applied")
	return nil
}
