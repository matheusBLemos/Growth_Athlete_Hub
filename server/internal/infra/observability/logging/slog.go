// Package logging implementa port.Logger sobre o log/slog da stdlib, emitindo
// JSON estruturado em stdout (o caminho que o Datadog Agent coleta) e injetando
// o contexto de trace em cada registro para correlação log↔trace no Datadog.
package logging

import (
	"context"
	"encoding/binary"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/trace"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// Logger adapta um *slog.Logger à porta port.Logger.
type Logger struct {
	l *slog.Logger
}

var _ port.Logger = (*Logger)(nil)

// New cria um Logger JSON escrevendo em w (use os.Stdout em produção) no nível
// mínimo dado. O handler injeta automaticamente trace_id/span_id (formato OTel)
// e dd.trace_id/dd.span_id (formato Datadog) quando há um span ativo no context.
func New(w io.Writer, level slog.Level, base ...slog.Attr) *Logger {
	h := &traceHandler{
		Handler: slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}),
	}
	l := slog.New(h)
	if len(base) > 0 {
		args := make([]any, len(base))
		for i, a := range base {
			args[i] = a
		}
		l = l.With(args...)
	}
	return &Logger{l: l}
}

// ParseLevel converte "debug"|"info"|"warn"|"error" em slog.Level (default info).
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func (lg *Logger) Debug(ctx context.Context, msg string, args ...any) {
	lg.l.DebugContext(ctx, msg, args...)
}
func (lg *Logger) Info(ctx context.Context, msg string, args ...any) {
	lg.l.InfoContext(ctx, msg, args...)
}
func (lg *Logger) Warn(ctx context.Context, msg string, args ...any) {
	lg.l.WarnContext(ctx, msg, args...)
}
func (lg *Logger) Error(ctx context.Context, msg string, args ...any) {
	lg.l.ErrorContext(ctx, msg, args...)
}

// Slog expõe o *slog.Logger subjacente, útil para passar a libs que esperam
// slog (ex.: o logger global em cmd/*).
func (lg *Logger) Slog() *slog.Logger { return lg.l }

// traceHandler envolve um slog.Handler e adiciona os identificadores de trace do
// span ativo no context a cada registro, permitindo correlação no Datadog.
type traceHandler struct {
	slog.Handler
}

func (h *traceHandler) Handle(ctx context.Context, r slog.Record) error {
	sc := trace.SpanContextFromContext(ctx)
	if sc.IsValid() {
		tid := sc.TraceID()
		sid := sc.SpanID()
		r.AddAttrs(
			// Formato OTel (hex) — neutro a fornecedor.
			slog.String("trace_id", tid.String()),
			slog.String("span_id", sid.String()),
			// Formato Datadog (decimal dos 64 bits baixos) — correlação log↔APM.
			slog.String("dd.trace_id", lower64Decimal(tid[:])),
			slog.String("dd.span_id", uint64Decimal(sid[:])),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *traceHandler) WithGroup(name string) slog.Handler {
	return &traceHandler{Handler: h.Handler.WithGroup(name)}
}

// lower64Decimal interpreta os 8 bytes menos significativos de um trace ID de
// 16 bytes (big-endian) como uint64 e devolve em decimal — formato que o
// Datadog usa para correlacionar logs com traces OTLP.
func lower64Decimal(traceID []byte) string {
	if len(traceID) != 16 {
		return ""
	}
	return strconv.FormatUint(binary.BigEndian.Uint64(traceID[8:]), 10)
}

// uint64Decimal interpreta 8 bytes (big-endian) como uint64 em decimal.
func uint64Decimal(spanID []byte) string {
	if len(spanID) != 8 {
		return ""
	}
	return strconv.FormatUint(binary.BigEndian.Uint64(spanID), 10)
}
