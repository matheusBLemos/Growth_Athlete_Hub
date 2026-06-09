//go:build integration

package postgres

import (
	"os"
	"testing"
)

// TestMigrateAgainstRealDB aplica todas as migrations embutidas contra um
// Postgres/TimescaleDB real e valida idempotência (rodar duas vezes não erra e
// não reaplica). Gated pela tag `integration` e pela env TEST_DATABASE_URL;
// pulado quando ausente, para que `go test ./...` não exija banco.
//
// Exemplo:
//
//	docker run --rm -d -p 55432:5432 -e POSTGRES_PASSWORD=test \
//	  -e POSTGRES_USER=test -e POSTGRES_DB=test timescale/timescaledb:latest-pg16
//	TEST_DATABASE_URL=postgres://test:test@localhost:55432/test?sslmode=disable \
//	  go test -tags integration ./internal/infra/persistence/postgres/...
func TestMigrateAgainstRealDB(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	db, err := NewDB(dsn)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	defer db.Close()

	if err := Migrate(db); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if count == 0 {
		t.Fatal("expected schema_migrations to be populated after Migrate")
	}

	// Idempotência: segunda execução não deve aplicar nada nem erro.
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate (idempotent): %v", err)
	}
	var count2 int
	if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count2); err != nil {
		t.Fatalf("re-count schema_migrations: %v", err)
	}
	if count2 != count {
		t.Fatalf("schema_migrations changed on re-run: was %d, now %d", count, count2)
	}

	// As hypertables devem existir no catálogo do TimescaleDB.
	for _, table := range []string{"activities", "metrics", "daily_metric_aggregates"} {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM timescaledb_information.hypertables WHERE hypertable_name = $1`,
			table,
		).Scan(&n)
		if err != nil {
			t.Fatalf("query hypertables for %s: %v", table, err)
		}
		if n != 1 {
			t.Errorf("expected %s to be a hypertable, found %d entries", table, n)
		}
	}
}
