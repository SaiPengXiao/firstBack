package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Config holds application configuration from environment variables.
type Config struct {
	Port        string
	JWTSecret   string
	AllowOrigin string
	MySQLDSN    string
}

// Load reads configuration with sensible defaults for local development.
// In production (GIN_MODE=release), it enforces strict environment variable validation.
func Load() Config {
	isProduction := os.Getenv("GIN_MODE") == "release"

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		if isProduction {
			log.Fatal("FATAL: JWT_SECRET environment variable is required in production mode!")
		}
		secret = "dev-secret-change-in-production"
		log.Println("WARNING: Using default JWT_SECRET. This is only safe for local development.")
	}

	origin := os.Getenv("CORS_ALLOW_ORIGIN")
	if origin == "" {
		if isProduction {
			log.Fatal("FATAL: CORS_ALLOW_ORIGIN environment variable is required in production mode!")
		}
		origin = "http://localhost:5173"
		log.Println("WARNING: Using default CORS_ALLOW_ORIGIN (localhost). This is only safe for local development.")
	}

	mysqlDSN, err := resolveMySQLDSN()
	if err != nil {
		log.Fatalf("FATAL: MySQL config: %v", err)
	}
	if mysqlDSN == "" {
		log.Fatal("FATAL: MySQL is required. Set MYSQL_DSN, MYSQL_URL (or DATABASE_URL), or MYSQL_USER/MYSQL_HOST/MYSQL_DATABASE")
	}

	return Config{
		Port:        port,
		JWTSecret:   secret,
		AllowOrigin: origin,
		MySQLDSN:    mysqlDSN,
	}
}

func resolveMySQLDSN() (string, error) {
	if dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN")); dsn != "" {
		return dsn, nil
	}

	for _, key := range []string{"MYSQL_URL", "DATABASE_URL", "MYSQL_PUBLIC_URL"} {
		if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
			return mysqlURLToDSN(raw)
		}
	}

	user := envFirst("MYSQL_USER", "MYSQLUSER")
	pass := envFirst("MYSQL_PASSWORD", "MYSQLPASSWORD", "MYSQL_ROOT_PASSWORD")
	host := envFirst("MYSQL_HOST", "MYSQLHOST")
	dbName := envFirst("MYSQL_DATABASE", "MYSQLDATABASE")
	port := envFirst("MYSQL_PORT", "MYSQLPORT")
	if port == "" {
		port = "3306"
	}

	if user != "" && host != "" && dbName != "" {
		return buildMySQLDSN(user, pass, host, port, dbName, mysqlUseTLS(host)), nil
	}

	return "", nil
}

func envFirst(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// mysqlURLToDSN converts Railway-style mysql://user:pass@host:port/db to go-sql-driver DSN.
func mysqlURLToDSN(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse mysql url: %w", err)
	}
	if u.Scheme != "mysql" {
		return "", fmt.Errorf("mysql url: expected scheme mysql, got %q", u.Scheme)
	}

	user := u.User.Username()
	pass, _ := u.User.Password()
	host := u.Hostname()
	port := u.Port()
	if port == "" {
		port = "3306"
	}
	database := strings.TrimPrefix(u.Path, "/")
	if database == "" {
		return "", fmt.Errorf("mysql url: missing database name in path")
	}

	return buildMySQLDSN(user, pass, host, port, database, mysqlUseTLS(host)), nil
}

func mysqlUseTLS(host string) bool {
	if v := strings.TrimSpace(os.Getenv("MYSQL_TLS")); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	h := strings.ToLower(strings.TrimSpace(host))
	switch h {
	case "127.0.0.1", "localhost", "::1":
		return false
	default:
		return strings.Contains(h, ".") // remote host (e.g. *.rlwy.net)
	}
}

func buildMySQLDSN(user, password, host, port, database string, useTLS bool) string {
	userInfo := url.UserPassword(user, password)
	q := "parseTime=true&loc=UTC&charset=utf8mb4&multiStatements=true"
	if useTLS {
		// skip-verify: Railway 等自签证书无法用 tls=true 校验（见 database/mysql.go RegisterTLSConfig）
		q += "&tls=skip-verify"
	}
	return fmt.Sprintf("%s@tcp(%s:%s)/%s?%s",
		userInfo.String(), host, port, database, q)
}

// PortInt returns the listen port as an integer.
func (c Config) PortInt() int {
	p, err := strconv.Atoi(c.Port)
	if err != nil {
		return 8080
	}
	return p
}