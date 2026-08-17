package task

import (
	"encoding/json"
	"testing"
)

func TestNewExampleTask(t *testing.T) {
	task, err := NewExampleTask("demo", "trace-1")
	if err != nil {
		t.Fatalf("NewExampleTask: %v", err)
	}
	if task.Type() != TypeExampleTask {
		t.Fatalf("Type = %q, want %q", task.Type(), TypeExampleTask)
	}

	var p ExamplePayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Name != "demo" {
		t.Fatalf("Name = %q, want demo", p.Name)
	}
	if p.TraceID != "trace-1" {
		t.Fatalf("TraceID = %q, want trace-1", p.TraceID)
	}
}

func TestNewExampleTaskWithoutTrace(t *testing.T) {
	task, err := NewExampleTask("demo")
	if err != nil {
		t.Fatalf("NewExampleTask: %v", err)
	}
	var p ExamplePayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.TraceID != "" {
		t.Fatalf("TraceID = %q, want empty", p.TraceID)
	}
}

func TestNewExampleTaskBlankTrace(t *testing.T) {
	task, err := NewExampleTask("demo", "  ")
	if err != nil {
		t.Fatalf("NewExampleTask: %v", err)
	}
	var p ExamplePayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.TraceID != "" {
		t.Fatalf("TraceID = %q, want empty for blank input", p.TraceID)
	}
}

func TestExamplePayloadOmitEmptyTrace(t *testing.T) {
	raw, err := json.Marshal(ExamplePayload{Name: "demo"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(raw) == `{"name":"demo","trace_id":""}` {
		t.Fatal("expected trace_id to be omitted when empty")
	}
}

func TestTraceIDFromPayload(t *testing.T) {
	payload, _ := json.Marshal(ExamplePayload{Name: "demo", TraceID: "t-9"})
	if got := TraceIDFromPayload(payload); got != "t-9" {
		t.Fatalf("TraceIDFromPayload = %q, want t-9", got)
	}
}

func TestTraceIDFromPayloadInvalidJSON(t *testing.T) {
	if got := TraceIDFromPayload([]byte("{not json")); got != "" {
		t.Fatalf("TraceIDFromPayload(invalid) = %q, want empty", got)
	}
}

func TestTraceIDFromPayloadEmpty(t *testing.T) {
	if got := TraceIDFromPayload(nil); got != "" {
		t.Fatalf("TraceIDFromPayload(nil) = %q, want empty", got)
	}
}
