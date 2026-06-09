// Command seed popula o banco com dois usuários PREVISÍVEIS, prontos para login,
// e seus dados de telemetria calibrados — substituindo o antigo seed.sql (que
// inseria usuários sem senha, impossíveis de autenticar).
//
// Diferença essencial vs. o SQL: as senhas são geradas pelo MESMO hasher
// Argon2id + pepper que a API usa (lendo o pepper do config), então os usuários
// semeados realmente conseguem fazer login via POST /api/v1/auth/login.
//
// Credenciais criadas:
//
//	maria.atleta@example.com  / SenhaForte123  -> atleta sobrecarregado (5 insights)
//	joao.saudavel@example.com / SenhaForte123  -> atleta saudável       (0 insights)
//
// É IDEMPOTENTE: limpa os dados destes dois usuários antes de reinserir. As
// datas das métricas/atividades são RELATIVAS a now() para se manterem dentro
// da janela de 30 dias usada pelo endpoint de insights.
//
// Uso:
//
//	DATABASE_URL=postgres://gah:gah@localhost:5432/gah?sslmode=disable go run ./cmd/seed
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/auth"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/config"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"
)

// IDs FIXOS, referenciáveis nos testes manuais (sprint_2.http / smoke_test.sh).
const (
	athleteID = "user-athlete-001"
	healthyID = "user-healthy-002"

	athleteEmail = "maria.atleta@example.com"
	healthyEmail = "joao.saudavel@example.com"
	// seedPassword é a senha em texto puro das duas contas semeadas.
	seedPassword = "SenhaForte123"
)

func main() {
	cfg, err := config.Load("config.toml")
	if err != nil {
		log.Fatalf("seed: failed to load config: %v", err)
	}

	db, err := postgres.NewDB(cfg.Database.URL)
	if err != nil {
		log.Fatalf("seed: failed to connect to database: %v", err)
	}
	defer db.Close()

	// Garante o schema (idempotente) antes de qualquer insert.
	if err := postgres.Migrate(db); err != nil {
		log.Fatalf("seed: failed to run migrations: %v", err)
	}

	// Hasher com o MESMO pepper que a API usa — senhas ficam compatíveis com login.
	hasher := auth.NewArgon2Hasher(cfg.Auth.PasswordPepper)
	userRepo := postgres.NewUserRepository(db)

	ctx := context.Background()

	if err := cleanup(ctx, db); err != nil {
		log.Fatalf("seed: cleanup failed: %v", err)
	}

	maria, err := buildUser(athleteID, "Maria Atleta", athleteEmail, "1995-03-12T00:00:00Z", hasher)
	if err != nil {
		log.Fatalf("seed: build maria: %v", err)
	}
	joao, err := buildUser(healthyID, "Joao Saudavel", healthyEmail, "1990-07-25T00:00:00Z", hasher)
	if err != nil {
		log.Fatalf("seed: build joao: %v", err)
	}

	if err := userRepo.Save(ctx, maria); err != nil {
		log.Fatalf("seed: save maria: %v", err)
	}
	if err := userRepo.Save(ctx, joao); err != nil {
		log.Fatalf("seed: save joao: %v", err)
	}

	athleteActivities, err := seedAthlete(ctx, db)
	if err != nil {
		log.Fatalf("seed: athlete data: %v", err)
	}
	healthyActivities, err := seedHealthy(ctx, db)
	if err != nil {
		log.Fatalf("seed: healthy data: %v", err)
	}

	printSummary(ctx, db, athleteActivities, healthyActivities)
}

// buildUser cria a entidade User com ID fixo e hash de senha real.
func buildUser(id, name, email, birthRFC3339 string, hasher *auth.Argon2Hasher) (*entity.User, error) {
	birth, err := time.Parse(time.RFC3339, birthRFC3339)
	if err != nil {
		return nil, err
	}
	hash, err := hasher.Hash(seedPassword)
	if err != nil {
		return nil, err
	}
	u, err := entity.NewUserWithCredentials(name, email, hash, birth)
	if err != nil {
		return nil, err
	}
	// Sobrescreve o ID gerado por um fixo, para que os dados sejam referenciáveis.
	u.ID = id
	return u, nil
}

// cleanup remove os dados dos dois usuários semeados, tornando o seed idempotente.
// Respeita as FKs: insights/metrics/activities -> users.
func cleanup(ctx context.Context, db *sql.DB) error {
	ids := []any{athleteID, healthyID}
	stmts := []string{
		`DELETE FROM insights   WHERE user_id IN ($1, $2)`,
		`DELETE FROM metrics    WHERE user_id IN ($1, $2)`,
		`DELETE FROM activities WHERE user_id IN ($1, $2)`,
		`DELETE FROM users      WHERE id      IN ($1, $2)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s, ids...); err != nil {
			return fmt.Errorf("%q: %w", s, err)
		}
	}
	return nil
}

// seedAthlete insere a atividade pré-existente e as métricas calibradas para
// disparar 5 insights (hrv_drop, resting_hr_high, sleep_deficit, overtraining,
// recovery_needed). Mesma calibração do antigo seed.sql. Retorna nº de atividades.
func seedAthlete(ctx context.Context, db *sql.DB) (int, error) {
	// Atividade pré-existente (external_id usado no teste de duplicidade 409).
	// duration_ns = 45 min.
	if _, err := db.ExecContext(ctx,
		`INSERT INTO activities (id, user_id, type, date, duration_ns, avg_heart_rate, external_id, created_at)
		 VALUES ('act-seed-001', $1, 'running', now() - interval '2 days', 2700000000000, 152, 'garmin-seed-001', now())`,
		athleteID,
	); err != nil {
		return 0, fmt.Errorf("athlete activity: %w", err)
	}

	bulk := []string{
		// HRV: atual=55, baseline=80 -> queda 31% (>= 15%).
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 SELECT 'm-ath-hrv-' || g, $1, 'hrv',
		        CASE WHEN g = 1 THEN 55 ELSE 80 END,
		        now() - make_interval(days => g), now()
		 FROM generate_series(1, 8) AS g`,
		// Resting HR: atual=60, baseline=48 -> alta 25% (>= 10%).
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 SELECT 'm-ath-rhr-' || g, $1, 'resting_hr',
		        CASE WHEN g = 1 THEN 60 ELSE 48 END,
		        now() - make_interval(days => g), now()
		 FROM generate_series(1, 8) AS g`,
		// Sleep: últimos 3 dias < 5h -> deficit CRÍTICO.
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 SELECT 'm-ath-sleep-' || g, $1, 'sleep_duration',
		        CASE WHEN g <= 3 THEN 4.5 ELSE 7.5 END,
		        now() - make_interval(days => g), now()
		 FROM generate_series(1, 7) AS g`,
		// Training load: ACWR = 120/60 = 2.0 -> overtraining CRÍTICO.
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 SELECT 'm-ath-load-' || g, $1, 'training_load',
		        CASE WHEN g <= 7 THEN 120 ELSE 40 END,
		        now() - make_interval(days => g), now()
		 FROM generate_series(1, 28) AS g`,
	}
	for _, q := range bulk {
		if _, err := db.ExecContext(ctx, q, athleteID); err != nil {
			return 0, fmt.Errorf("athlete metrics: %w", err)
		}
	}
	return 1, nil
}

// seedHealthy insere métricas estáveis e sem histórico suficiente de carga, de
// modo que o endpoint de insights retorne 0. Retorna nº de atividades (0).
func seedHealthy(ctx context.Context, db *sql.DB) (int, error) {
	bulk := []string{
		// HRV estável em 80 (sem queda).
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 SELECT 'm-hea-hrv-' || g, $1, 'hrv', 80,
		        now() - make_interval(days => g), now()
		 FROM generate_series(1, 8) AS g`,
		// Resting HR estável em 50 (sem alta).
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 SELECT 'm-hea-rhr-' || g, $1, 'resting_hr', 50,
		        now() - make_interval(days => g), now()
		 FROM generate_series(1, 8) AS g`,
		// Sono saudável em 8h.
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 SELECT 'm-hea-sleep-' || g, $1, 'sleep_duration', 8.0,
		        now() - make_interval(days => g), now()
		 FROM generate_series(1, 7) AS g`,
		// Carga estável, < 28 pontos -> regra ACWR não dispara.
		`INSERT INTO metrics (id, user_id, type, value, date, created_at)
		 SELECT 'm-hea-load-' || g, $1, 'training_load', 50,
		        now() - make_interval(days => g), now()
		 FROM generate_series(1, 10) AS g`,
	}
	for _, q := range bulk {
		if _, err := db.ExecContext(ctx, q, healthyID); err != nil {
			return 0, fmt.Errorf("healthy metrics: %w", err)
		}
	}
	return 0, nil
}

func printSummary(ctx context.Context, db *sql.DB, athleteActs, healthyActs int) {
	count := func(table, userID string) int {
		var n int
		// table é uma constante de chamada interna (não vem de input externo).
		_ = db.QueryRowContext(ctx,
			fmt.Sprintf(`SELECT count(*) FROM %s WHERE user_id = $1`, table), userID,
		).Scan(&n)
		return n
	}

	fmt.Println("=================================================")
	fmt.Println("GAH seed aplicado com sucesso")
	fmt.Println("=================================================")
	fmt.Println("Usuários criados (prontos para login):")
	fmt.Printf("  - %s  (id=%s)\n", athleteEmail, athleteID)
	fmt.Printf("      senha: %s   perfil: atleta sobrecarregado (5 insights)\n", seedPassword)
	fmt.Printf("      métricas=%d  atividades=%d\n", count("metrics", athleteID), athleteActs)
	fmt.Printf("  - %s (id=%s)\n", healthyEmail, healthyID)
	fmt.Printf("      senha: %s   perfil: atleta saudável (0 insights)\n", seedPassword)
	fmt.Printf("      métricas=%d  atividades=%d\n", count("metrics", healthyID), healthyActs)
	fmt.Println("=================================================")
}
