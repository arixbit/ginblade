//go:build integration

package integration_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/arixbit/ginblade/pkg/cache"
)

func TestRedisCacheIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := cache.NewClient(cache.RedisConfig{
		Addr:     requireEnv(t, "TEST_REDIS_ADDR"),
		Password: os.Getenv("TEST_REDIS_PASSWORD"),
		DB:       requireIntEnv(t, "TEST_REDIS_CACHE_DB"),
	})
	if err != nil {
		t.Fatalf("connect to redis cache: %v", err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Errorf("close redis cache: %v", err)
		}
	})
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping redis cache: %v", err)
	}

	prefix := "ginblade:integration:" + strings.ReplaceAll(uuid.NewString(), "-", "") + ":"
	keys := make([]string, 0, 3)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), dependencyTimeout)
		defer cancel()
		if len(keys) > 0 {
			if err := client.Underlying().Del(cleanupCtx, keys...).Err(); err != nil {
				t.Errorf("delete redis integration keys: %v", err)
			}
		}
	})

	missing, err := client.Get(ctx, prefix+"missing")
	if err != nil {
		t.Fatalf("get missing redis key: %v", err)
	}
	if missing != "" {
		t.Fatalf("missing redis value = %q, want empty", missing)
	}

	cases := []struct {
		name  string
		value string
		ttl   time.Duration
	}{
		{name: "persistent", value: "without-expiration", ttl: 0},
		{name: "expiring", value: "with-expiration", ttl: 30 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key := prefix + tc.name
			keys = append(keys, key)
			if err := client.Set(ctx, key, tc.value, tc.ttl); err != nil {
				t.Fatalf("set redis key: %v", err)
			}
			got, err := client.Get(ctx, key)
			if err != nil {
				t.Fatalf("get redis key: %v", err)
			}
			if got != tc.value {
				t.Fatalf("redis value = %q, want %q", got, tc.value)
			}

			gotTTL, err := client.Underlying().PTTL(ctx, key).Result()
			if err != nil {
				t.Fatalf("read redis TTL: %v", err)
			}
			if tc.ttl == 0 && gotTTL >= 0 {
				t.Fatalf("persistent redis TTL = %s, want no expiration", gotTTL)
			}
			if tc.ttl > 0 && (gotTTL <= 0 || gotTTL > tc.ttl) {
				t.Fatalf("expiring redis TTL = %s, want > 0 and <= %s", gotTTL, tc.ttl)
			}
		})
	}
}
