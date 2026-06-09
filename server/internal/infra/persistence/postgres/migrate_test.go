package postgres

import (
	"testing"
	"testing/fstest"
)

// TestMigrationVersionsOrderingAndParsing valida a lógica de descoberta,
// parsing de versão e ordenação lexicográfica das migrations, sem banco.
func TestMigrationVersionsOrderingAndParsing(t *testing.T) {
	// Inserimos fora de ordem de propósito para provar a ordenação.
	fsys := fstest.MapFS{
		"010_later.sql":    {Data: []byte("SELECT 10;")},
		"001_initial.sql":  {Data: []byte("SELECT 1;")},
		"002_password.sql": {Data: []byte("SELECT 2;")},
		"notes.txt":        {Data: []byte("ignore me")}, // não-.sql deve ser ignorado
		"003_provider.sql": {Data: []byte("SELECT 3;")},
	}

	versions, err := migrationVersions(fsys)
	if err != nil {
		t.Fatalf("migrationVersions: %v", err)
	}

	want := []migrationFile{
		{version: "001_initial", filename: "001_initial.sql"},
		{version: "002_password", filename: "002_password.sql"},
		{version: "003_provider", filename: "003_provider.sql"},
		{version: "010_later", filename: "010_later.sql"},
	}

	if len(versions) != len(want) {
		t.Fatalf("got %d versions, want %d: %+v", len(versions), len(want), versions)
	}
	for i := range want {
		if versions[i] != want[i] {
			t.Errorf("versions[%d] = %+v, want %+v", i, versions[i], want[i])
		}
	}
}

// TestEmbeddedMigrationsDiscoverable garante que os arquivos reais embutidos
// (server/migrations/*.sql) são encontrados e ordenados, protegendo contra um
// embed quebrado.
func TestEmbeddedMigrationsDiscoverable(t *testing.T) {
	versions, err := migrationVersions(migrationsFS())
	if err != nil {
		t.Fatalf("migrationVersions(embedded): %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("expected at least one embedded migration, got none")
	}
	// A primeira deve ser o schema inicial.
	if versions[0].version != "001_initial_schema" {
		t.Errorf("first migration = %q, want 001_initial_schema", versions[0].version)
	}
	// Devem estar ordenadas crescentemente.
	for i := 1; i < len(versions); i++ {
		if versions[i-1].version >= versions[i].version {
			t.Errorf("migrations not sorted: %q before %q", versions[i-1].version, versions[i].version)
		}
	}
}
