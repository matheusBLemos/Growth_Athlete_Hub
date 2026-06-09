package usecase

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// Estratégia de invalidação do cache de leitura de métricas
// ----------------------------------------------------------
// As chaves de query são namespaced por uma versão por usuário:
//
//	metrics:{userID}:v{ver}:{metricType}:{from}:{to}
//
// A versão fica numa chave própria: metrics:{userID}:ver.
//
// Na ESCRITA (RecordMetric) bumpamos a versão do usuário gravando um novo valor
// (UnixNano monotônico). Como as chaves de query carregam a versão antiga,
// todas elas passam a "não existir" para as próximas leituras — invalidação
// O(1), sem precisar enumerar/escanear chaves. As entradas órfãs da versão
// anterior expiram naturalmente pelo TTL.
//
// Resiliência: qualquer erro de cache é tratado como miss/no-op pelos callers;
// a falta de versão (cache miss na chave :ver) é tratada como versão 0.

// metricsVersionKey é a chave que guarda a versão de cache do usuário.
func metricsVersionKey(userID string) string {
	return fmt.Sprintf("metrics:%s:ver", userID)
}

// metricsVersion lê a versão de cache atual do usuário. Cache miss ou erro
// resultam em versão 0 (degradação graciosa).
func metricsVersion(ctx context.Context, cache port.Cache, userID string) string {
	if cache == nil {
		return "0"
	}
	v, hit, err := cache.Get(ctx, metricsVersionKey(userID))
	if err != nil || !hit {
		return "0"
	}
	return string(v)
}

// metricsQueryKey monta a chave determinística de uma query, embutindo a versão.
func metricsQueryKey(userID, version, metricType string, from, to time.Time) string {
	return fmt.Sprintf(
		"metrics:%s:v%s:%s:%d:%d",
		userID, version, metricType, from.UnixNano(), to.UnixNano(),
	)
}

// bumpMetricsVersion invalida todas as queries cacheadas do usuário gravando uma
// nova versão (UnixNano). É O(1) e não exige enumeração de chaves.
func bumpMetricsVersion(ctx context.Context, cache port.Cache, userID string, ttl time.Duration) error {
	if cache == nil {
		return nil
	}
	ver := strconv.FormatInt(time.Now().UnixNano(), 10)
	return cache.Set(ctx, metricsVersionKey(userID), []byte(ver), ttl)
}
