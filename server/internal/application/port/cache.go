package port

import (
	"context"
	"time"
)

// Cache é a porta de cache chave-valor usada pela camada de aplicação.
// É intencionalmente byte-oriented: os chamadores controlam a serialização
// (JSON, etc.), mantendo a porta agnóstica ao formato do valor armazenado.
type Cache interface {
	// Get retorna o valor da chave. O segundo retorno (hit) é false quando a
	// chave não existe; nesse caso value é nil e err é nil.
	Get(ctx context.Context, key string) (value []byte, hit bool, err error)
	// Set grava value sob key com o TTL informado. TTL <= 0 grava sem expiração.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Delete remove uma ou mais chaves. Remover chave inexistente não é erro.
	Delete(ctx context.Context, keys ...string) error
}
