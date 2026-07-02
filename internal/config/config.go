package config

import (
	"log"
	"os"
	"strconv"
)

// Config holds application configuration from environment variables.
type Config struct {
	Port        string
	JWTSecret   string
	AllowOrigin string
	SQLitePath  string
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

	sqlitePath := os.Getenv("SQLITE_PATH")
	if sqlitePath == "" {
		sqlitePath = "data/app.db"
	}

	return Config{
		Port:        port,
		JWTSecret:   secret,
		AllowOrigin: origin,
		SQLitePath:  sqlitePath,
	}
}

// PortInt returns the listen port as an integer.
func (c Config) PortInt() int {
	p, err := strconv.Atoi(c.Port)
	if err != nil {
		return 8080
	}
	return p
}