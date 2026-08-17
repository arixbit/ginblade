package database

import (
	"testing"
	"time"

	"gorm.io/gorm/logger"
)

func TestInitEmptyDSN(t *testing.T) {
	m, err := Init(Config{})
	if err != nil {
		t.Fatalf("Init with empty DSN should not error, got %v", err)
	}
	if m.DB() != nil {
		t.Fatal("expected nil DB for empty DSN")
	}
	if err := m.Ping(t.Context()); err == nil {
		t.Fatal("expected ping error for unconfigured database")
	}
	if err := m.Close(); err != nil {
		t.Fatalf("Close on unconfigured database should be a no-op, got %v", err)
	}
}

func TestNewTestManager(t *testing.T) {
	m := NewTestManager(nil)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	if m.DB() != nil {
		t.Fatal("expected nil DB")
	}
}

func TestNormalizePoolSettingsDefaults(t *testing.T) {
	got := normalizePoolSettings(Config{})
	if got.maxIdleConns != 15 {
		t.Fatalf("maxIdleConns = %d, want 15", got.maxIdleConns)
	}
	if got.maxOpenConns != 30 {
		t.Fatalf("maxOpenConns = %d, want 30", got.maxOpenConns)
	}
	if got.connMaxLifetime != 30*time.Minute {
		t.Fatalf("connMaxLifetime = %v, want 30m", got.connMaxLifetime)
	}
	if got.connMaxIdleTime != 5*time.Minute {
		t.Fatalf("connMaxIdleTime = %v, want 5m", got.connMaxIdleTime)
	}
}

func TestNormalizePoolSettingsKeepsCustomValues(t *testing.T) {
	cfg := Config{
		MaxIdleConns:    3,
		MaxOpenConns:    7,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: 30 * time.Second,
	}
	got := normalizePoolSettings(cfg)
	if got.maxIdleConns != 3 || got.maxOpenConns != 7 {
		t.Fatalf("pool sizes not preserved: %+v", got)
	}
	if got.connMaxLifetime != time.Minute || got.connMaxIdleTime != 30*time.Second {
		t.Fatalf("pool durations not preserved: %+v", got)
	}
}

func TestParseLogLevel(t *testing.T) {
	cases := []struct {
		in   string
		want logger.LogLevel
	}{
		{"silent", logger.Silent},
		{"error", logger.Error},
		{"info", logger.Info},
		{"warn", logger.Warn},
		{"warning", logger.Warn},
		{"", logger.Warn},
		{"WARN", logger.Warn},
		{"unknown", logger.Warn},
	}
	for _, tc := range cases {
		if got := parseLogLevel(tc.in); got != tc.want {
			t.Errorf("parseLogLevel(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
