package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/tiendv89/workflow-backend/cmd/api"
	"github.com/tiendv89/workflow-backend/cmd/migration"
	"github.com/tiendv89/workflow-backend/configs"
)

var (
	cfgFile string
	cfg     *configs.Config
)

var rootCmd = &cobra.Command{
	Use:   "api-service",
	Short: "workflow-backend API service",
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "configs/config.yaml", "path to config file")
	if err := rootCmd.MarkPersistentFlagRequired("config"); err != nil {
		panic(err)
	}

	cobra.OnInitialize(initConfig)

	rootCmd.AddCommand(api.NewCommand(&cfg))
	rootCmd.AddCommand(migration.NewCommand(&cfg))
}

func initConfig() {
	loaded, err := configs.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	cfg = loaded

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	level, err := zerolog.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
