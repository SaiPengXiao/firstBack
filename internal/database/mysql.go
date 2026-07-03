package database

import (
	"crypto/tls"
	"database/sql"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
)

func init() {
	// Railway 等托管 MySQL 常用自签证书，无法用 tls=true 校验；DSN 使用 tls=skip-verify
	_ = mysql.RegisterTLSConfig("skip-verify", &tls.Config{InsecureSkipVerify: true})
}

// OpenMySQL opens a MySQL connection pool and runs migrations.
// dsn examples:
//   user:pass@tcp(127.0.0.1:3306)/firstback?parseTime=true&loc=UTC&charset=utf8mb4
func OpenMySQL(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, fmt.Errorf("mysql: empty DSN")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping mysql: %w", err)
	}

	if err := migrateMySQL(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func migrateMySQL(db *sql.DB) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
	id CHAR(36) PRIMARY KEY,
	username VARCHAR(191) NOT NULL,
	email VARCHAR(191) NOT NULL,
	password_hash VARBINARY(255) NOT NULL,
	created_at DATETIME(3) NOT NULL,
	UNIQUE KEY uk_users_username (username),
	UNIQUE KEY uk_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS menu_categories (
	id CHAR(36) PRIMARY KEY,
	name VARCHAR(64) NOT NULL,
	sort_order INT NOT NULL DEFAULT 0,
	created_at DATETIME(3) NOT NULL,
	UNIQUE KEY uk_menu_categories_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS menu_items (
	id CHAR(36) PRIMARY KEY,
	category_id CHAR(36) NOT NULL,
	name VARCHAR(128) NOT NULL,
	description TEXT,
	price DECIMAL(10,2) NOT NULL DEFAULT 0.00,
	image_url VARCHAR(512) DEFAULT NULL,
	is_available TINYINT(1) NOT NULL DEFAULT 1,
	sort_order INT NOT NULL DEFAULT 0,
	created_at DATETIME(3) NOT NULL,
	updated_at DATETIME(3) NOT NULL,
	KEY idx_menu_items_category (category_id),
	KEY idx_menu_items_available (is_available),
	CONSTRAINT fk_menu_items_category FOREIGN KEY (category_id) REFERENCES menu_categories(id) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("migrate mysql: %w", err)
	}
	return nil
}