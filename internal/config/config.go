package config

import (
	"fmt"
	"log"
	"os"
)

// Config holds all application configuration.
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	RedisHost  string
	RedisPort  string
	ServerPort string
	JWTSecret  string
}

// Load reads configuration from environment variables.
// In non-development environments (APP_ENV != "development") DB_PASSWORD and
// JWT_SECRET must be explicitly provided; the service will not start otherwise.
func Load() *Config {
	isDev := os.Getenv("APP_ENV") == "development"

	dbPassword := os.Getenv("DB_PASSWORD")
	jwtSecret := os.Getenv("JWT_SECRET")

	if isDev {
		if dbPassword == "" {
			dbPassword = "skyhigh123"
		}
		if jwtSecret == "" {
			jwtSecret = "skyhigh-secret-key"
		}
	} else {
		if dbPassword == "" {
			log.Fatal("DB_PASSWORD environment variable is required")
		}
		if jwtSecret == "" {
			log.Fatal("JWT_SECRET environment variable is required")
		}
	}

	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBUser:     getEnv("DB_USER", "skyhigh"),
		DBPassword: dbPassword,
		DBName:     getEnv("DB_NAME", "skyhigh_db"),
		RedisHost:  getEnv("REDIS_HOST", "localhost"),
		RedisPort:  getEnv("REDIS_PORT", "6379"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		JWTSecret:  jwtSecret,
	}
}

// DSN returns the PostgreSQL data source name.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable TimeZone=UTC",
		c.DBHost, c.DBPort, c.DBUser, c.DBPassword, c.DBName,
	)
}

// RedisAddr returns the Redis address string.
func (c *Config) RedisAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
