//go:build integration

package postgres_test

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"
)

// openTestDB abre o banco apontado por TEST_DATABASE_URL e aplica as migrations.
// Cada suíte de integração pula (t.Skip) quando a env não está configurada, de
// modo que infra parcial ainda roda o que for possível.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}

	db, err := postgres.NewDB(dsn)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := postgres.Migrate(db); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

// uniqueID gera um sufixo aleatório para evitar colisões entre execuções/suites
// no mesmo banco (os testes não truncam tabelas compartilhadas).
func uniqueID(prefix string) string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s-%x", prefix, b)
}
