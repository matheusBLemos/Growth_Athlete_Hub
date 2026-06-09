package port

import "context"

// Logger é a porta de logging estruturado da camada de aplicação. Mantém os use
// cases desacoplados de qualquer biblioteca concreta (slog, zap, etc.) e do
// formato de saída. Os args seguem o estilo chave/valor do slog
// (ex.: log.Info(ctx, "msg", "key", value, "other", n)).
//
// As implementações devem extrair o contexto de trace do ctx e correlacionar o
// log à requisição/span correspondente.
type Logger interface {
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
}

// loggerCtxKey é a chave (não exportada) sob a qual o Logger viaja no context.
type loggerCtxKey struct{}

// nopLogger é o Logger padrão quando nenhum foi injetado no context. Descarta
// tudo silenciosamente, garantindo que LoggerFromContext nunca retorne nil.
type nopLogger struct{}

func (nopLogger) Debug(context.Context, string, ...any) {}
func (nopLogger) Info(context.Context, string, ...any)  {}
func (nopLogger) Warn(context.Context, string, ...any)  {}
func (nopLogger) Error(context.Context, string, ...any) {}

// ContextWithLogger devolve um context derivado carregando o Logger. A borda da
// aplicação (HTTP middleware, worker) o injeta uma vez; os use cases o recuperam
// via LoggerFromContext, sem precisar recebê-lo no construtor.
func ContextWithLogger(ctx context.Context, l Logger) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerCtxKey{}, l)
}

// LoggerFromContext recupera o Logger do context. Retorna um no-op quando
// nenhum foi injetado — nunca nil, então o chamador pode usá-lo direto.
func LoggerFromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(loggerCtxKey{}).(Logger); ok {
		return l
	}
	return nopLogger{}
}
