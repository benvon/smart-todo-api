package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Logging  LoggingConfig
	Security SecurityConfig
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port         string
	Host         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// DatabaseConfig holds database-specific configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// RedisConfig holds Redis-specific configuration
type RedisConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
	PoolSize int
}

// LoggingConfig holds logging-specific configuration
type LoggingConfig struct {
	Level  string
	Format string
	File   string
}

// SecurityConfig holds security-specific configuration
type SecurityConfig struct {
	JWTSecret    string
	JWTExpiry    time.Duration
	BCryptCost   int
	APIKey       string
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         getEnv("APP_PORT", "8080"),
			Host:         getEnv("APP_HOST", "0.0.0.0"),
			ReadTimeout:  getEnvAsDuration("APP_READ_TIMEOUT", 15*time.Second),
			WriteTimeout: getEnvAsDuration("APP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:  getEnvAsDuration("APP_IDLE_TIMEOUT", 60*time.Second),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvAsInt("DB_PORT", 5432),
			Name:            getEnv("DB_NAME", "app_db"),
			User:            getEnv("DB_USER", "app_user"),
			Password:        getEnv("DB_PASSWORD", "app_password"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvAsInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvAsInt("DB_MAX_IDLE_CONNS", 25),
			ConnMaxLifetime: getEnvAsDuration("DB_CONN_MAX_LIFETIME", 300*time.Second),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
			PoolSize: getEnvAsInt("REDIS_POOL_SIZE", 10),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
			File:   getEnv("LOG_FILE", ""),
		},
		Security: SecurityConfig{
			JWTSecret:  getEnv("JWT_SECRET", "your-secret-key"),
			JWTExpiry:  getEnvAsDuration("JWT_EXPIRY", 24*time.Hour),
			BCryptCost: getEnvAsInt("BCRYPT_COST", 12),
			APIKey:     getEnv("API_KEY", ""),
		},
	}

	return cfg, nil
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as an integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvAsDuration gets an environment variable as a duration or returns a default value
func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// String returns a string representation of the configuration
func (c *Config) String() string {
	return fmt.Sprintf("Config{Server:%s, Database:%s, Redis:%s, Logging:%s, Security:%s}",
		c.Server.String(),
		c.Database.String(),
		c.Redis.String(),
		c.Logging.String(),
		c.Security.String())
}

func (s ServerConfig) String() string {
	return fmt.Sprintf("ServerConfig{Port:%s, Host:%s}", s.Port, s.Host)
}

func (d DatabaseConfig) String() string {
	return fmt.Sprintf("DatabaseConfig{Host:%s, Port:%d, Name:%s}", d.Host, d.Port, d.Name)
}

func (r RedisConfig) String() string {
	return fmt.Sprintf("RedisConfig{Host:%s, Port:%d}", r.Host, r.Port)
}

func (l LoggingConfig) String() string {
	return fmt.Sprintf("LoggingConfig{Level:%s, Format:%s}", l.Level, l.Format)
}

func (s SecurityConfig) String() string {
	return fmt.Sprintf("SecurityConfig{JWTExpiry:%s, BCryptCost:%d}", s.JWTExpiry, s.BCryptCost)
}
