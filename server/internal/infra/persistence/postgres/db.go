package postgres

import (
	"database/sql"
	"time"

	"github.com/XSAM/otelsql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"go.opentelemetry.io/otel/attribute"
)

func NewDB(dsn string) (*sql.DB, error) {
	// otelsql.Open instrumenta cada query com spans/métricas via os providers
	// globais do OpenTelemetry. Quando a telemetria está desabilitada, os
	// providers são no-op — overhead desprezível.
	db, err := otelsql.Open("pgx", dsn, otelsql.WithAttributes(
		attribute.String("db.system", "postgresql"),
	))
	if err != nil {
		return nil, err
	}

	// Registra métricas do pool de conexões (db.sql.*) no meter global.
	if _, err := otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(
		attribute.String("db.system", "postgresql"),
	)); err != nil {
		db.Close()
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
