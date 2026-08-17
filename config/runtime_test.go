package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEnvMissingFileNoError(t *testing.T) {
	LoadEnv(filepath.Join(t.TempDir(), "does-not-exist.env")) // must not panic
}

func TestLoadEnvFallsBackToRootDotEnv(t *testing.T) {
	// LoadEnv always appends ".env"; loading a missing explicit path followed
	// by a missing root ".env" must be a no-op.
	LoadEnv(filepath.Join(t.TempDir(), "x.env"))
}

func TestLoadEnvReadsFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "test.env")
	if err := os.WriteFile(envFile, []byte("LOADENV_TEST_KEY=hello\n"), 0o644); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	os.Unsetenv("LOADENV_TEST_KEY")
	t.Cleanup(func() { os.Unsetenv("LOADENV_TEST_KEY") })

	LoadEnv(envFile)

	if got := os.Getenv("LOADENV_TEST_KEY"); got != "hello" {
		t.Fatalf("LOADENV_TEST_KEY = %q, want hello", got)
	}
}
