package config

import (
	"os"
	"testing"
	"time"
)

// clearEnv removes the given environment variables for the test and restores
// them afterwards.
func clearEnv(t *testing.T, keys ...string) {
	t.Helper()
	saved := make(map[string]string, len(keys))
	for _, k := range keys {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})
}

var allEnvKeys = []string{
	"SERVER_PORT", "GIN_MODE", "TRUSTED_PROXIES", "REQUEST_TIMEOUT",
	"POSTGRES", "GORM_LOG_LEVEL", "DB_MAX_IDLE_CONNS", "DB_MAX_OPEN_CONNS",
	"DB_CONN_MAX_LIFETIME", "DB_CONN_MAX_IDLE_TIME",
	"REDIS_ADDR", "REDIS_PASSWORD", "REDIS_CACHE_DB", "REDIS_QUEUE_DB",
	"JWT_SECRET", "JWT_ISSUER", "JWT_TTL",
	"CORS_ALLOW_ORIGINS",
	"LOG_LEVEL", "LOG_FORMAT", "LOG_STACKTRACE_LEVEL",
	"AUDIT_LOG_ENABLED", "AUDIT_LOG_EXCLUDE_PATHS",
	"RATE_LIMIT_PER_MINUTE",
}

func TestLoadDefaults(t *testing.T) {
	clearEnv(t, allEnvKeys...)

	cfg := Load()

	if cfg.Server.Port != ":3000" {
		t.Errorf("Port = %q, want :3000", cfg.Server.Port)
	}
	if cfg.Server.GinMode != "release" {
		t.Errorf("GinMode = %q, want release", cfg.Server.GinMode)
	}
	if cfg.Server.RequestTimeout != 30*time.Second {
		t.Errorf("RequestTimeout = %v, want 30s", cfg.Server.RequestTimeout)
	}
	if cfg.Postgres.MaxIdleConns != 15 {
		t.Errorf("MaxIdleConns = %d, want 15", cfg.Postgres.MaxIdleConns)
	}
	if cfg.Postgres.MaxOpenConns != 30 {
		t.Errorf("MaxOpenConns = %d, want 30", cfg.Postgres.MaxOpenConns)
	}
	if cfg.Postgres.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("ConnMaxLifetime = %v, want 30m", cfg.Postgres.ConnMaxLifetime)
	}
	if cfg.Redis.CacheDB != 0 {
		t.Errorf("CacheDB = %d, want 0", cfg.Redis.CacheDB)
	}
	if cfg.Redis.QueueDB != 6 {
		t.Errorf("QueueDB = %d, want 6", cfg.Redis.QueueDB)
	}
	if cfg.Auth.JWTIssuer != "ginblade" {
		t.Errorf("JWTIssuer = %q, want ginblade", cfg.Auth.JWTIssuer)
	}
	if cfg.Auth.JWTTTL != 24*time.Hour {
		t.Errorf("JWTTTL = %v, want 24h", cfg.Auth.JWTTTL)
	}
	if cfg.Log.Level != "info" || cfg.Log.Format != "json" {
		t.Errorf("Log = %+v, want info/json", cfg.Log)
	}
	if cfg.Log.StacktraceLevel != "error" {
		t.Errorf("StacktraceLevel = %q, want error", cfg.Log.StacktraceLevel)
	}
	if !cfg.Log.AuditEnabled {
		t.Error("AuditEnabled = false, want true")
	}
	if cfg.RateLimit.RequestsPerMinute != 0 {
		t.Errorf("RequestsPerMinute = %d, want 0", cfg.RateLimit.RequestsPerMinute)
	}
}

func TestLoadFromEnv(t *testing.T) {
	clearEnv(t, allEnvKeys...)
	t.Setenv("SERVER_PORT", ":8080")
	t.Setenv("REQUEST_TIMEOUT", "5s")
	t.Setenv("DB_MAX_OPEN_CONNS", "10")
	t.Setenv("REDIS_CACHE_DB", "3")
	t.Setenv("REDIS_QUEUE_DB", "9")
	t.Setenv("JWT_ISSUER", "my-service")
	t.Setenv("JWT_TTL", "1h")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("AUDIT_LOG_ENABLED", "false")
	t.Setenv("RATE_LIMIT_PER_MINUTE", "100")
	t.Setenv("CORS_ALLOW_ORIGINS", "a.com, b.com")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 10.0.0.2")
	t.Setenv("AUDIT_LOG_EXCLUDE_PATHS", "/health, /metrics")

	cfg := Load()

	if cfg.Server.Port != ":8080" {
		t.Errorf("Port = %q, want :8080", cfg.Server.Port)
	}
	if cfg.Server.RequestTimeout != 5*time.Second {
		t.Errorf("RequestTimeout = %v, want 5s", cfg.Server.RequestTimeout)
	}
	if cfg.Postgres.MaxOpenConns != 10 {
		t.Errorf("MaxOpenConns = %d, want 10", cfg.Postgres.MaxOpenConns)
	}
	if cfg.Redis.CacheDB != 3 || cfg.Redis.QueueDB != 9 {
		t.Errorf("Redis DBs = %d/%d, want 3/9", cfg.Redis.CacheDB, cfg.Redis.QueueDB)
	}
	if cfg.Auth.JWTIssuer != "my-service" || cfg.Auth.JWTTTL != time.Hour {
		t.Errorf("Auth = %+v, want my-service/1h", cfg.Auth)
	}
	if cfg.Log.Format != "console" || cfg.Log.AuditEnabled {
		t.Errorf("Log = %+v, want console/false", cfg.Log)
	}
	if cfg.RateLimit.RequestsPerMinute != 100 {
		t.Errorf("RequestsPerMinute = %d, want 100", cfg.RateLimit.RequestsPerMinute)
	}
	if len(cfg.Cors.AllowOrigins) != 2 || cfg.Cors.AllowOrigins[0] != "a.com" {
		t.Errorf("AllowOrigins = %v, want [a.com b.com]", cfg.Cors.AllowOrigins)
	}
	if len(cfg.Server.TrustedProxies) != 2 {
		t.Errorf("TrustedProxies = %v, want 2 entries", cfg.Server.TrustedProxies)
	}
	if len(cfg.Log.AuditExcludes) != 2 {
		t.Errorf("AuditExcludes = %v, want 2 entries", cfg.Log.AuditExcludes)
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	clearEnv(t, "TEST_GETENV")
	if got := getEnvOrDefault("TEST_GETENV", "fallback"); got != "fallback" {
		t.Errorf("unset = %q, want fallback", got)
	}
	t.Setenv("TEST_GETENV", "value")
	if got := getEnvOrDefault("TEST_GETENV", "fallback"); got != "value" {
		t.Errorf("set = %q, want value", got)
	}
	t.Setenv("TEST_GETENV", "  ")
	if got := getEnvOrDefault("TEST_GETENV", "fallback"); got != "fallback" {
		t.Errorf("blank = %q, want fallback", got)
	}
}

func TestParseCSV(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b", []string{"a", "b"}},
		{" a , b ,, c ", []string{"a", "b", "c"}},
		{",,,", nil},
	}
	for _, tc := range cases {
		got := parseCSV(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("parseCSV(%q) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("parseCSV(%q) = %v, want %v", tc.in, got, tc.want)
				break
			}
		}
	}
}

func TestIntEnv(t *testing.T) {
	clearEnv(t, "TEST_INT")
	if got := intEnv("TEST_INT", 7); got != 7 {
		t.Errorf("unset = %d, want 7", got)
	}
	t.Setenv("TEST_INT", "42")
	if got := intEnv("TEST_INT", 7); got != 42 {
		t.Errorf("set = %d, want 42", got)
	}
	t.Setenv("TEST_INT", "abc")
	if got := intEnv("TEST_INT", 7); got != 7 {
		t.Errorf("invalid = %d, want 7", got)
	}
}

func TestBoolEnv(t *testing.T) {
	clearEnv(t, "TEST_BOOL")
	if got := boolEnv("TEST_BOOL", true); !got {
		t.Error("unset = false, want true (default)")
	}
	t.Setenv("TEST_BOOL", "false")
	if got := boolEnv("TEST_BOOL", true); got {
		t.Error("set false = true, want false")
	}
	t.Setenv("TEST_BOOL", "bogus")
	if got := boolEnv("TEST_BOOL", true); !got {
		t.Error("invalid = false, want true (default)")
	}
}

func TestDurationEnv(t *testing.T) {
	clearEnv(t, "TEST_DUR")
	if got := durationEnv("TEST_DUR", time.Minute); got != time.Minute {
		t.Errorf("unset = %v, want 1m", got)
	}
	t.Setenv("TEST_DUR", "90s")
	if got := durationEnv("TEST_DUR", time.Minute); got != 90*time.Second {
		t.Errorf("set = %v, want 90s", got)
	}
	t.Setenv("TEST_DUR", "bogus")
	if got := durationEnv("TEST_DUR", time.Minute); got != time.Minute {
		t.Errorf("invalid = %v, want 1m", got)
	}
}
