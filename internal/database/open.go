package database

import (
	"database/sql"

	"firstgo-back/internal/config"
)

// Open connects to MySQL using configuration from cfg.
func Open(cfg config.Config) (*sql.DB, error) {
	return OpenMySQL(cfg.MySQLDSN)
}