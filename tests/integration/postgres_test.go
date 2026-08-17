//go:build integration

package integration_test

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/model"
	"github.com/arixbit/ginblade/internal/repository"
	"github.com/arixbit/ginblade/pkg/database"
)

func TestPostgresRepositoryIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := newIsolatedPostgres(t, ctx)
	db := manager.DB()

	if err := manager.Ping(ctx); err != nil {
		t.Fatalf("ping isolated postgres: %v", err)
	}
	if err := db.WithContext(ctx).AutoMigrate(&model.Example{}); err != nil {
		t.Fatalf("migrate examples table: %v", err)
	}

	repo := repository.NewExampleRepository(db)
	cases := []struct {
		name  string
		value string
	}{
		{name: "alpha", value: "alpha"},
		{name: "beta", value: "beta"},
		{name: "gamma", value: "gamma"},
	}
	created := make([]model.Example, 0, len(cases))
	for _, tc := range cases {
		t.Run("create_"+tc.name, func(t *testing.T) {
			example := model.Example{Name: tc.value}
			if err := repo.Create(ctx, &example); err != nil {
				t.Fatalf("create example: %v", err)
			}
			if example.ID == 0 {
				t.Fatal("created example has zero ID")
			}
			created = append(created, example)
		})
	}

	for i := 1; i < len(created); i++ {
		if created[i].ID <= created[i-1].ID {
			t.Fatalf("example IDs are not increasing: %d then %d", created[i-1].ID, created[i].ID)
		}
	}

	examples, total, err := repo.List(ctx, 2, 1)
	if err != nil {
		t.Fatalf("list examples: %v", err)
	}
	if total != 3 {
		t.Fatalf("list total = %d, want 3", total)
	}
	if len(examples) != 2 || examples[0].Name != "beta" || examples[1].Name != "alpha" {
		t.Fatalf("paginated names = %#v, want beta then alpha", exampleNames(examples))
	}

	t.Run("transaction_commit", func(t *testing.T) {
		err := repository.InTx(ctx, db, func(txCtx context.Context) error {
			return repo.Create(txCtx, &model.Example{Name: "committed"})
		})
		if err != nil {
			t.Fatalf("commit transaction: %v", err)
		}
		assertExampleCount(t, ctx, db, "committed", 1)
	})

	t.Run("transaction_rollback", func(t *testing.T) {
		rollbackErr := errors.New("force rollback")
		err := repository.InTx(ctx, db, func(txCtx context.Context) error {
			if err := repo.Create(txCtx, &model.Example{Name: "rolled-back"}); err != nil {
				return err
			}
			return rollbackErr
		})
		if !errors.Is(err, rollbackErr) {
			t.Fatalf("rollback error = %v, want %v", err, rollbackErr)
		}
		assertExampleCount(t, ctx, db, "rolled-back", 0)
	})

	_, total, err = repo.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("list examples after transactions: %v", err)
	}
	if total != 4 {
		t.Fatalf("total after commit and rollback = %d, want 4", total)
	}
}

func newIsolatedPostgres(t *testing.T, ctx context.Context) *database.DBManager {
	t.Helper()

	baseDSN := requireEnv(t, "TEST_POSTGRES_DSN")
	admin, err := database.Init(testDatabaseConfig(baseDSN))
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Close(); err != nil {
			t.Errorf("close postgres admin connection: %v", err)
		}
	})
	if err := admin.Ping(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	schema := "go_skeleton_it_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err := admin.DB().WithContext(ctx).Exec("CREATE SCHEMA " + quoteIdentifier(schema)).Error; err != nil {
		t.Fatalf("create postgres schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), dependencyTimeout)
		defer cancel()
		if err := admin.DB().WithContext(cleanupCtx).Exec("DROP SCHEMA IF EXISTS " + quoteIdentifier(schema) + " CASCADE").Error; err != nil {
			t.Errorf("drop postgres schema %s: %v", schema, err)
		}
	})

	manager, err := database.Init(testDatabaseConfig(postgresDSNWithSchema(t, baseDSN, schema)))
	if err != nil {
		t.Fatalf("connect to isolated postgres schema %s: %v", schema, err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("close isolated postgres connection: %v", err)
		}
	})

	var currentSchema string
	if err := manager.DB().WithContext(ctx).Raw("SELECT current_schema()").Scan(&currentSchema).Error; err != nil {
		t.Fatalf("read current postgres schema: %v", err)
	}
	if currentSchema != schema {
		t.Fatalf("current postgres schema = %q, want %q", currentSchema, schema)
	}

	sqlDB, err := manager.DB().DB()
	if err != nil {
		t.Fatalf("get postgres connection pool: %v", err)
	}
	if got := sqlDB.Stats().MaxOpenConnections; got != 2 {
		t.Fatalf("max open postgres connections = %d, want 2", got)
	}
	return manager
}

func testDatabaseConfig(dsn string) database.Config {
	return database.Config{
		DSN:             dsn,
		LogLevel:        "silent",
		MaxIdleConns:    1,
		MaxOpenConns:    2,
		ConnMaxLifetime: time.Minute,
		ConnMaxIdleTime: time.Minute,
	}
}

func postgresDSNWithSchema(t *testing.T, rawDSN, schema string) string {
	t.Helper()

	dsn, err := url.Parse(rawDSN)
	if err != nil || dsn.Scheme == "" || dsn.Host == "" {
		t.Fatalf("TEST_POSTGRES_DSN must be a PostgreSQL URL: %v", err)
	}
	query := dsn.Query()
	query.Set("search_path", schema)
	dsn.RawQuery = query.Encode()
	return dsn.String()
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func exampleNames(examples []model.Example) []string {
	names := make([]string, 0, len(examples))
	for _, example := range examples {
		names = append(names, example.Name)
	}
	return names
}

func assertExampleCount(t *testing.T, ctx context.Context, db *gorm.DB, name string, want int64) {
	t.Helper()

	var got int64
	if err := db.WithContext(ctx).Model(&model.Example{}).Where("name = ?", name).Count(&got).Error; err != nil {
		t.Fatalf("count example %q: %v", name, err)
	}
	if got != want {
		t.Fatalf("example %q count = %d, want %d", name, got, want)
	}
}
