package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Load reads all configuration from environment variables. Call after LoadEnv.
func Load() *Config {
	port := getEnvOrDefault("SERVER_PORT", ":3000")
	ginMode := getEnvOrDefault("GIN_MODE", "release")
	logLevel := getEnvOrDefault("LOG_LEVEL", "info")
	logFormat := getEnvOrDefault("LOG_FORMAT", "json")
	stacktraceLevel := getEnvOrDefault("LOG_STACKTRACE_LEVEL", "error")

	return &Config{
		Server: ServerConfig{
			Port:           port,
			GinMode:        ginMode,
			TrustedProxies: parseCSV(os.Getenv("TRUSTED_PROXIES")),
			RequestTimeout: durationEnv("REQUEST_TIMEOUT", 30*time.Second),
		},
		Postgres: PostgresConfig{
			DSN:             os.Getenv("POSTGRES"),
			LogLevel:        os.Getenv("GORM_LOG_LEVEL"),
			MaxIdleConns:    intEnv("DB_MAX_IDLE_CONNS", 15),
			MaxOpenConns:    intEnv("DB_MAX_OPEN_CONNS", 30),
			ConnMaxLifetime: durationEnv("DB_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: durationEnv("DB_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:     os.Getenv("REDIS_ADDR"),
			Password: os.Getenv("REDIS_PASSWORD"),
			CacheDB:  intEnv("REDIS_CACHE_DB", 0),
			QueueDB:  intEnv("REDIS_QUEUE_DB", 6),
		},
		Auth: AuthConfig{
			JWTSecret: os.Getenv("JWT_SECRET"),
			JWTIssuer: getEnvOrDefault("JWT_ISSUER", "ginblade"),
			JWTTTL:    durationEnv("JWT_TTL", 24*time.Hour),
		},
		Cors: CorsConfig{
			AllowOrigins: parseCSV(os.Getenv("CORS_ALLOW_ORIGINS")),
		},
		Log: LogConfig{
			Level:           logLevel,
			Format:          logFormat,
			StacktraceLevel: stacktraceLevel,
			AuditEnabled:    boolEnv("AUDIT_LOG_ENABLED", true),
			AuditExcludes:   parseCSV(os.Getenv("AUDIT_LOG_EXCLUDE_PATHS")),
		},
		RateLimit: RateLimitConfig{
			RequestsPerMinute: intEnv("RATE_LIMIT_PER_MINUTE", 0),
		},
	}
}

func getEnvOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func parseCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func intEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
