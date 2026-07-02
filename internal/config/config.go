package config

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strconv"
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

	mysqlDSN := os.Getenv("MYSQL_DSN")
	if mysqlDSN == "" {
		user := os.Getenv("MYSQL_USER")
		pass := os.Getenv("MYSQL_PASSWORD")
		host := os.Getenv("MYSQL_HOST")
		dbName := os.Getenv("MYSQL_DATABASE")
		if user != "" && host != "" && dbName != "" {
			mysqlPort := os.Getenv("MYSQL_PORT")
			if mysqlPort == "" {
				mysqlPort = "3306"
			}
			mysqlDSN = buildMySQLDSN(user, pass, host, mysqlPort, dbName)
		}
	}

	if mysqlDSN == "" {
		log.Fatal("FATAL: MySQL is required. Set MYSQL_DSN or MYSQL_USER, MYSQL_PASSWORD, MYSQL_HOST, MYSQL_DATABASE")
	}

	return Config{
		Port:        port,
		JWTSecret:   secret,
		AllowOrigin: origin,
		MySQLDSN:    mysqlDSN,
	}
}

func buildMySQLDSN(user, password, host, port, database string) string {
	userInfo := url.UserPassword(user, password)
	return fmt.Sprintf("%s@tcp(%s:%s)/%s?parseTime=true&loc=UTC&charset=utf8mb4&multiStatements=true",
		userInfo.String(), host, port, database)
}

// PortInt returns the listen port as an integer.
func (c Config) PortInt() int {
	p, err := strconv.Atoi(c.Port)
	if err != nil {
		return 8080
	}
	return p
}