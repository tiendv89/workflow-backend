package db

import "fmt"

type Config struct {
	Host                string `mapstructure:"host" json:"host"`
	Port                int    `mapstructure:"port" json:"port"`
	DBName              string `mapstructure:"db_name" json:"db_name"`
	User                string `mapstructure:"user" json:"user"`
	Password            string `mapstructure:"password" json:"password"`
	ConnLifeTimeSeconds int    `mapstructure:"conn_life_time_seconds" json:"conn_life_time_seconds"`
	MaxIdleConns        int    `mapstructure:"max_idle_conns" json:"max_idle_conns"`
	MaxOpenConns        int    `mapstructure:"max_open_conns" json:"max_open_conns"`
	LogLevel            int    `mapstructure:"log_level" json:"log_level"`
	AutoMigration       bool   `mapstructure:"auto_migration" json:"auto_migration"`
	MigrationDir        string `mapstructure:"migration_dir" json:"migration_dir"`
}

func (c *Config) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		c.User, c.Password, c.Host, c.Port, c.DBName,
	)
}
