package log

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
)

func TestInitJSONLogger(t *testing.T) {
	logger, err := Init(Config{Level: "info", Format: "json", Service: "test"})
	if err != nil {
		t.Fatalf("Init(json): %v", err)
	}
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
	restore := SetLogger(zap.NewNop())
	defer restore()
}

func TestInitConsoleLogger(t *testing.T) {
	if _, err := Init(Config{Level: "debug", Format: "console"}); err != nil {
		t.Fatalf("Init(console): %v", err)
	}
	restore := SetLogger(zap.NewNop())
	defer restore()
}

func TestInitUnsupportedFormat(t *testing.T) {
	if _, err := Init(Config{Level: "info", Format: "yaml"}); err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestInitInvalidLevel(t *testing.T) {
	if _, err := Init(Config{Level: "not-a-level", Format: "json"}); err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestSetLoggerRestore(t *testing.T) {
	before := L()
	restore := SetLogger(zap.NewNop())
	if L() == before {
		t.Fatal("expected logger to change after SetLogger")
	}
	restore()
	if L() != before {
		t.Fatal("expected logger to be restored")
	}
}

func TestTraceIDRoundTrip(t *testing.T) {
	ctx := WithTraceID(context.Background(), "trace-abc")
	if got := TraceIDFrom(ctx); got != "trace-abc" {
		t.Fatalf("TraceIDFrom = %q, want trace-abc", got)
	}
}

func TestTraceIDFromNilContext(t *testing.T) {
	if got := TraceIDFrom(nil); got != "" {
		t.Fatalf("TraceIDFrom(nil) = %q, want empty", got)
	}
}

func TestTraceIDFromEmptyContext(t *testing.T) {
	if got := TraceIDFrom(context.Background()); got != "" {
		t.Fatalf("TraceIDFrom(background) = %q, want empty", got)
	}
}

func TestEnsureTraceIDKeepsExisting(t *testing.T) {
	ctx := WithTraceID(context.Background(), "existing")
	out := EnsureTraceID(ctx, "new")
	if got := TraceIDFrom(out); got != "existing" {
		t.Fatalf("EnsureTraceID overwrote trace: got %q, want existing", got)
	}
}

func TestEnsureTraceIDSetsMissing(t *testing.T) {
	out := EnsureTraceID(context.Background(), "new")
	if got := TraceIDFrom(out); got != "new" {
		t.Fatalf("EnsureTraceID = %q, want new", got)
	}
}

func TestEnsureTraceIDIgnoresEmpty(t *testing.T) {
	out := EnsureTraceID(context.Background(), "  ")
	if got := TraceIDFrom(out); got != "" {
		t.Fatalf("EnsureTraceID with empty value = %q, want empty", got)
	}
}

func TestNewTraceID(t *testing.T) {
	got := NewTraceID("api", "", "task", "  ")
	if got != "api:task" {
		t.Fatalf("NewTraceID = %q, want api:task", got)
	}
}

func TestNewTraceIDAllEmpty(t *testing.T) {
	if got := NewTraceID("", " "); got != "" {
		t.Fatalf("NewTraceID(all empty) = %q, want empty", got)
	}
}

func TestFromContextWithoutTrace(t *testing.T) {
	logger := FromContext(context.Background())
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestFromContextWithTrace(t *testing.T) {
	ctx := WithTraceID(context.Background(), "t-1")
	logger := FromContext(ctx)
	if logger == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestParseLevel(t *testing.T) {
	if _, err := parseLevel("debug"); err != nil {
		t.Fatalf("parseLevel(debug): %v", err)
	}
	if _, err := parseLevel(""); err != nil {
		t.Fatalf("parseLevel(empty) should default to info: %v", err)
	}
	if _, err := parseLevel("bogus"); err == nil {
		t.Fatal("expected error for bogus level")
	}
}

func TestErrorField(t *testing.T) {
	field := Error(errors.New("boom"))
	if field == (zap.Field{}) {
		t.Fatal("expected non-empty zap field")
	}
}
