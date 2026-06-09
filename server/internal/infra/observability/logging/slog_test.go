package logging

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"log/slog"
	"strconv"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

// decodeLine decodifica a única linha JSON escrita pelo logger no buffer.
func decodeLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &m); err != nil {
		t.Fatalf("decode log line: %v (raw=%q)", err, buf.String())
	}
	return m
}

func TestLogger_InjectsTraceContext(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	tid, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	sid, _ := trace.SpanIDFromHex("1112131415161718")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	logger.Info(ctx, "hello", "user_id", "u-1")

	m := decodeLine(t, &buf)

	if got := m["trace_id"]; got != tid.String() {
		t.Errorf("trace_id = %v, want %s", got, tid.String())
	}
	if got := m["span_id"]; got != sid.String() {
		t.Errorf("span_id = %v, want %s", got, sid.String())
	}

	wantDDTrace := strconv.FormatUint(binary.BigEndian.Uint64(tid[8:]), 10)
	if got := m["dd.trace_id"]; got != wantDDTrace {
		t.Errorf("dd.trace_id = %v, want %s", got, wantDDTrace)
	}
	wantDDSpan := strconv.FormatUint(binary.BigEndian.Uint64(sid[:]), 10)
	if got := m["dd.span_id"]; got != wantDDSpan {
		t.Errorf("dd.span_id = %v, want %s", got, wantDDSpan)
	}

	if got := m["user_id"]; got != "u-1" {
		t.Errorf("user_id = %v, want u-1", got)
	}
	if got := m["msg"]; got != "hello" {
		t.Errorf("msg = %v, want hello", got)
	}
}

func TestLogger_NoTraceContext_OmitsTraceFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelInfo)

	logger.Info(context.Background(), "no trace")

	m := decodeLine(t, &buf)
	if _, ok := m["trace_id"]; ok {
		t.Errorf("trace_id should be absent without a span, got %v", m["trace_id"])
	}
	if _, ok := m["dd.trace_id"]; ok {
		t.Errorf("dd.trace_id should be absent without a span")
	}
}

func TestLogger_RespectsLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(&buf, slog.LevelWarn)

	logger.Info(context.Background(), "filtered")
	if buf.Len() != 0 {
		t.Errorf("info log should be filtered at warn level, got %q", buf.String())
	}

	logger.Warn(context.Background(), "kept")
	if buf.Len() == 0 {
		t.Error("warn log should pass at warn level")
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"":      slog.LevelInfo,
		"bogus": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := ParseLevel(in); got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
