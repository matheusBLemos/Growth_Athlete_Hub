package postgres

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	"github.com/Growth-Athlete-Hub/gah-server/migrations"
)

// Migrate aplica as migrations SQL embutidas (server/migrations/*.sql) de forma
// rastreada e idempotente: cada versão já aplicada é registrada em
// schema_migrations e pulada nas execuções seguintes. Pode ser chamada com
// segurança no boot de cada processo (api, worker, migrate).
func Migrate(db *sql.DB) error {
	return MigrateFS(db, migrations.Files)
}

// migrationsFS devolve o fs.FS embutido com os arquivos reais de migration.
func migrationsFS() fs.FS {
	return migrations.Files
}

// MigrateFS é a variante testável que recebe o fs.FS de origem dos arquivos
// *.sql, permitindo injetar um sistema de arquivos falso em testes.
func MigrateFS(db *sql.DB, fsys fs.FS) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return fmt.Errorf("load applied versions: %w", err)
	}

	versions, err := migrationVersions(fsys)
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	pending := 0
	for _, v := range versions {
		if applied[v.version] {
			continue
		}
		sqlBytes, err := fs.ReadFile(fsys, v.filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", v.filename, err)
		}
		if err := applyMigration(db, v.version, string(sqlBytes)); err != nil {
			return fmt.Errorf("apply migration %s: %w", v.filename, err)
		}
		log.Printf("migrations: applied %s", v.filename)
		pending++
	}

	if pending == 0 {
		log.Println("migrations: database is up to date (no pending migrations)")
	} else {
		log.Printf("migrations: %d migration(s) applied", pending)
	}
	return nil
}

// migrationFile associa o nome do arquivo embutido à sua versão (o nome do
// arquivo, ex.: "001_initial_schema.sql"). Mantemos o nome completo como versão
// para evitar ambiguidades de parsing — basta ser estável e ordenável.
type migrationFile struct {
	version  string
	filename string
}

// migrationVersions lê todos os *.sql do fs.FS e os devolve ordenados
// lexicograficamente (001_, 002_, ...).
func migrationVersions(fsys fs.FS) ([]migrationFile, error) {
	entries, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return nil, err
	}
	sort.Strings(entries)

	files := make([]migrationFile, 0, len(entries))
	for _, name := range entries {
		files = append(files, migrationFile{
			version:  strings.TrimSuffix(name, ".sql"),
			filename: name,
		})
	}
	return files, nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`)
	return err
}

func appliedVersions(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

// applyMigration roda uma migration na sua própria transação e, no mesmo commit,
// registra a versão em schema_migrations — garantindo que o tracking só avance
// se o SQL inteiro tiver sucesso.
func applyMigration(db *sql.DB, version, sqlText string) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op após commit bem-sucedido

	if _, err := tx.Exec(sqlText); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`INSERT INTO schema_migrations (version) VALUES ($1)`, version,
	); err != nil {
		return err
	}
	return tx.Commit()
}
