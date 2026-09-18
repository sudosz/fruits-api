package config

import (
	"fmt"
	"os"
)

// Config holds every runtime setting, all of it sourced from the environment.
type Config struct {
	Port string

	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	DBSSLMode  string
}

// Load reads configuration from the environment, falling back to development defaults.
func Load() Config {
	return Config{
		Port: env("PORT", "8080"),

		DBHost:     env("DB_HOST", "localhost"),
		DBPort:     env("DB_PORT", "5432"),
		DBUser:     env("DB_USER", "postgres"),
		DBPassword: env("DB_PASSWORD", "postgres"),
		DBName:     env("DB_NAME", "fruitsdb"),
		DBSSLMode:  env("DB_SSLMODE", "disable"),
	}
}

// DSN builds the PostgreSQL connection string. DATABASE_URL, when set, wins so
// that platforms which only hand out a single URL work without extra mapping.
func (c Config) DSN() string {
	if url := os.Getenv("DATABASE_URL"); url != "" {
		return url
	}

	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName, c.DBSSLMode,
	)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
