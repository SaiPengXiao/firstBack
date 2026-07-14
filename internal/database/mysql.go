package database

import (
	"crypto/tls"
	"database/sql"
	"errors"
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

	const ordersSchema = `
CREATE TABLE IF NOT EXISTS orders (
  id CHAR(36) PRIMARY KEY,
  user_id CHAR(36) NOT NULL,
  note VARCHAR(255) DEFAULT NULL,
  total_amount DECIMAL(10,2) NOT NULL DEFAULT 0.00,
  created_at DATETIME(3) NOT NULL,
  CONSTRAINT fk_orders_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT,
  KEY idx_orders_user (user_id),
  KEY idx_orders_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS order_items (
  id CHAR(36) PRIMARY KEY,
  order_id CHAR(36) NOT NULL,
  menu_item_id CHAR(36) NOT NULL,
  name VARCHAR(128) NOT NULL,
  price DECIMAL(10,2) NOT NULL,
  quantity INT NOT NULL,
  CONSTRAINT fk_oi_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
  KEY idx_oi_order (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
`
	if _, err := db.Exec(ordersSchema); err != nil {
		return fmt.Errorf("migrate mysql orders: %w", err)
	}
	if err := alterMigrateMySQL(db); err != nil {
		return fmt.Errorf("alter migrate mysql: %w", err)
	}
	return nil
}

// alterMigrateMySQL applies incremental schema changes idempotently.
func alterMigrateMySQL(db *sql.DB) error {
	// users table additions (idempotent: ignore "Duplicate column name" 1060)
	userAlters := []string{
		`ALTER TABLE users ADD COLUMN openid VARCHAR(128) NULL`,
		`ALTER TABLE users ADD COLUMN unionid VARCHAR(128) NULL`,
		`ALTER TABLE users ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'active'`,
		`ALTER TABLE users ADD COLUMN last_login_at DATETIME(3) NULL`,
		`ALTER TABLE users ADD COLUMN updated_at DATETIME(3) NULL`,
	}
	for _, stmt := range userAlters {
		if _, err := db.Exec(stmt); err != nil && !isDupColumnOrIndex(err) {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}

	// openid unique index (NULLs don't trigger unique constraint, safe for old accounts)
	indexStmts := []string{
		`CREATE UNIQUE INDEX uk_users_openid ON users(openid)`,
	}
	for _, stmt := range indexStmts {
		if _, err := db.Exec(stmt); err != nil && !isDupColumnOrIndex(err) {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}

	// orders table additions
	orderAlters := []string{
		`ALTER TABLE orders ADD COLUMN table_no VARCHAR(20) NULL`,
		`ALTER TABLE orders ADD COLUMN updated_at DATETIME(3) NULL`,
	}
	for _, stmt := range orderAlters {
		if _, err := db.Exec(stmt); err != nil && !isDupColumnOrIndex(err) {
			return fmt.Errorf("%s: %w", stmt, err)
		}
	}
	return nil
}

func isDupColumnOrIndex(err error) bool {
	if err == nil {
		return false
	}
	var me *mysql.MySQLError
	// 1060 = Duplicate column name, 1061 = Duplicate key name
	return errors.As(err, &me) && (me.Number == 1060 || me.Number == 1061)
}