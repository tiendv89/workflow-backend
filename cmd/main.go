package main

import (
	_ "net/http/pprof"
	"os"

	"github.com/fatih/color"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/tiendv89/workflow-backend/cmd/api"
	"github.com/tiendv89/workflow-backend/cmd/migration"
	"github.com/tiendv89/workflow-backend/configs"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:          "api-service",
	Short:        "workflow-backend API service",
	SilenceUsage: true,
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal().Err(err).Msg("Engine failed to start")
	}
}

func initLogging() {
	logLevel, err := zerolog.ParseLevel(configs.G.Log.Level)
	if err != nil {
		logLevel = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(logLevel)
	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000000"

	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: "01-02 15:04:05.000000",
		FormatLevel: func(i interface{}) string {
			if lvl, ok := i.(string); ok {
				switch lvl {
				case "warn":
					return color.RedString("[WARN]")
				case "info":
					return color.GreenString("[INFO]")
				case "error":
					return color.RedString("[ERROR]")
				case "debug":
					return color.BlueString("[DEBUG]")
				default:
					return color.WhiteString("[%s]", lvl)
				}
			}
			return color.CyanString("[UNKNOWN]")
		},
	}

	multiWriter := zerolog.MultiLevelWriter(consoleWriter)
	log.Logger = zerolog.New(multiWriter).With().Timestamp().Logger()
}

func init() {
	cobra.OnInitialize(func() { configs.Init(cfgFile) })
	cobra.OnInitialize(initLogging)

	rootCmd.AddCommand(api.Command)
	rootCmd.AddCommand(migration.Command)

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (required)")
	if err := rootCmd.MarkFlagRequired("config"); err != nil {
		return
	}
}
