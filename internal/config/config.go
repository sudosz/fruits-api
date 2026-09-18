package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
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

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration
	DBConnMaxIdleTime time.Duration
	DBConnectTimeout  time.Duration
	DBMigrateTimeout  time.Duration

	RateLimitPerSecond float64
	RateLimitBurst     int
	RateLimitReapEvery time.Duration
	MaxBodyBytes       int64

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxHeaderBytes    int
	ShutdownTimeout   time.Duration
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

		DBMaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 25),
		DBConnMaxLifetime: envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		DBConnMaxIdleTime: envDuration("DB_CONN_MAX_IDLE_TIME", time.Minute),
		DBConnectTimeout:  envDuration("DB_CONNECT_TIMEOUT", 30*time.Second),
		DBMigrateTimeout:  envDuration("DB_MIGRATE_TIMEOUT", 15*time.Second),

		RateLimitPerSecond: envFloat("RATE_LIMIT_PER_SECOND", 50),
		RateLimitBurst:     envInt("RATE_LIMIT_BURST", 100),
		RateLimitReapEvery: envDuration("RATE_LIMIT_REAP_INTERVAL", 10*time.Minute),
		MaxBodyBytes:       int64(envInt("MAX_BODY_BYTES", 8<<10)),

		ReadHeaderTimeout: envDuration("SERVER_READ_HEADER_TIMEOUT", 5*time.Second),
		ReadTimeout:       envDuration("SERVER_READ_TIMEOUT", 15*time.Second),
		WriteTimeout:      envDuration("SERVER_WRITE_TIMEOUT", 15*time.Second),
		IdleTimeout:       envDuration("SERVER_IDLE_TIMEOUT", 60*time.Second),
		MaxHeaderBytes:    envInt("SERVER_MAX_HEADER_BYTES", 1<<20),
		ShutdownTimeout:   envDuration("SERVER_SHUTDOWN_TIMEOUT", 10*time.Second),
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

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil || f < 0 {
		return fallback
	}
	return f
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
