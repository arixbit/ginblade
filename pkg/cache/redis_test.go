package cache

import (
	"context"
	"testing"
	"time"
)

func TestNewClientEmptyAddr(t *testing.T) {
	if _, err := NewClient(RedisConfig{Addr: "  "}); err == nil {
		t.Fatal("expected error for empty address")
	}
}

func TestClientGetNotConfigured(t *testing.T) {
	c := &Client{}
	if _, err := c.Get(context.Background(), "key"); err == nil {
		t.Fatal("expected error for unconfigured client")
	}
}

func TestClientSetNotConfigured(t *testing.T) {
	c := &Client{}
	if err := c.Set(context.Background(), "key", "value", time.Minute); err == nil {
		t.Fatal("expected error for unconfigured client")
	}
}

func TestClientPingNotConfigured(t *testing.T) {
	c := &Client{}
	if err := c.Ping(context.Background()); err == nil {
		t.Fatal("expected error for unconfigured client")
	}
}

func TestClientCloseNotConfigured(t *testing.T) {
	c := &Client{}
	if err := c.Close(); err != nil {
		t.Fatalf("Close on unconfigured client should be a no-op, got %v", err)
	}
}

func TestNilClientMethods(t *testing.T) {
	var c *Client
	if err := c.Close(); err != nil {
		t.Fatalf("Close on nil client should be a no-op, got %v", err)
	}
	if err := c.Ping(context.Background()); err == nil {
		t.Fatal("expected error from nil client Ping")
	}
	if _, err := c.Get(context.Background(), "key"); err == nil {
		t.Fatal("expected error from nil client Get")
	}
	if err := c.Set(context.Background(), "key", "v", 0); err == nil {
		t.Fatal("expected error from nil client Set")
	}
}
