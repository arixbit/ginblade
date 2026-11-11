package cache

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewTestClientClose(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	client := NewTestClient(rdb)
	if err := client.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	err := rdb.Ping(context.Background()).Err()
	if err == nil || !strings.Contains(err.Error(), "client is closed") {
		t.Fatalf("underlying client not closed: %v", err)
	}
}

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
