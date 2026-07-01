package config

import (
	"os"
	"strconv"
)

// Config holds application configuration from environment variables.
type Config struct {
	Port        string
	JWTSecret   string
	AllowOrigin string
}

// Load reads configuration with sensible defaults for local development.
func Load() Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "dev-secret-change-in-production"
	}
	origin := os.Getenv("CORS_ALLOW_ORIGIN")
	if origin == "" {
		origin = "http://localhost:5173"
	}
	return Config{
		Port:        port,
		JWTSecret:   secret,
		AllowOrigin: origin,
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