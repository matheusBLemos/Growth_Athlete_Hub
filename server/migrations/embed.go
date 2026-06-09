// Package migrations expõe os arquivos SQL de migration embutidos no binário
// (via go:embed), para que o runner em internal/infra/persistence/postgres os
// aplique de forma rastreada (tabela schema_migrations) em qualquer ambiente —
// sem depender do docker-entrypoint-initdb.d do Postgres.
package migrations

import "embed"

// Files contém todos os arquivos *.sql desta pasta, embutidos no binário.
// A ordem de aplicação é lexicográfica (001_, 002_, ...).
//
//go:embed *.sql
var Files embed.FS
