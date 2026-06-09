// Command migrate aplica as migrations SQL embutidas (server/migrations/*.sql)
// no banco configurado e encerra. É o runner standalone: útil em CI, em jobs de
// deploy ou via `make migrate`. A api e o worker também rodam Migrate no boot,
// então este binário é opcional no fluxo normal de dev.
package main

import (
	"log"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/config"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"
)

func main() {
	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	db, err := postgres.NewDB(cfg.Database.URL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := postgres.Migrate(db); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}
	log.Println("migrate: done")
}
