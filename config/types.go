package config

import "time"

// Config holds all application configuration loaded once at startup.
type Config struct {
	Server    ServerConfig
	Postgres  PostgresConfig
	Redis     RedisConfig
	Auth      AuthConfig
	Cors      CorsConfig
	Log       LogConfig
	RateLimit RateLimitConfig
	I18n      I18nConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port           string
	GinMode        string
	TrustedProxies []string
	RequestTimeout time.Duration
}

// PostgresConfig holds database connection settings.
type PostgresConfig struct {
	DSN             string
	LogLevel        string
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// RedisConfig holds Redis settings for cache and queue clients.
type RedisConfig struct {
	Addr     string
	Password string
	CacheDB  int
	QueueDB  int
}

// AuthConfig holds JWT authentication settings.
type AuthConfig struct {
	JWTSecret string
	JWTIssuer string
	JWTTTL    time.Duration
}

// CorsConfig holds allowed browser origins.
type CorsConfig struct {
	AllowOrigins []string
}

// LogConfig holds structured logging settings.
type LogConfig struct {
	Level           string
	Format          string
	StacktraceLevel string
	AuditEnabled    bool
	AuditExcludes   []string
}

// RateLimitConfig holds per-IP rate limit settings.
type RateLimitConfig struct {
	RequestsPerMinute int
}

// I18nConfig holds response message localization settings.
type I18nConfig struct {
	DefaultLocale string
}
