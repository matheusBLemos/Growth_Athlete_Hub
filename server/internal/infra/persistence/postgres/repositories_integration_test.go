//go:build integration

// Testes de integração dos repositórios Postgres contra um banco TimescaleDB
// real (o schema com hypertables). Gated em TEST_DATABASE_URL — cada teste pula
// quando a env não está configurada. Usam IDs únicos por execução para não
// colidir em tabelas compartilhadas (que não são truncadas).
package postgres_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/persistence/postgres"
)

func TestIntegration_UserRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	repo := postgres.NewUserRepository(db)

	id := uniqueID("user")
	email := id + "@example.com"
	u := &entity.User{
		ID:           id,
		Name:         "Test User",
		Email:        email,
		PasswordHash: "$argon2id$v=19$m=1,t=1,p=1$abc$def",
		BirthDate:    time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:    time.Now().UTC(),
	}

	if err := repo.Save(ctx, u); err != nil {
		t.Fatalf("save user: %v", err)
	}

	byID, err := repo.FindByID(ctx, id)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if byID == nil || byID.Email != email {
		t.Fatalf("find by id mismatch: %+v", byID)
	}

	byEmail, err := repo.FindByEmail(ctx, email)
	if err != nil {
		t.Fatalf("find by email: %v", err)
	}
	if byEmail == nil || byEmail.ID != id {
		t.Fatalf("find by email mismatch: %+v", byEmail)
	}

	// Upsert: Save de novo deve atualizar, não duplicar.
	u.Name = "Updated Name"
	if err := repo.Save(ctx, u); err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	again, _ := repo.FindByID(ctx, id)
	if again.Name != "Updated Name" {
		t.Fatalf("upsert did not update name: %q", again.Name)
	}

	// Inexistente -> (nil, nil).
	none, err := repo.FindByID(ctx, "does-not-exist")
	if err != nil || none != nil {
		t.Fatalf("expected nil,nil for missing user; got %+v, %v", none, err)
	}
}

func TestIntegration_ActivityRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	userID := uniqueID("user")
	mustSaveUser(t, db, userID)

	repo := postgres.NewActivityRepository(db)
	now := time.Now().UTC()

	ext := uniqueID("ext")
	a, err := entity.NewActivity(userID, valueobject.ActivityTypeRunning, now.Add(-2*time.Hour), 45*time.Minute, 150, ext)
	if err != nil {
		t.Fatalf("new activity: %v", err)
	}
	if err := repo.Save(ctx, a); err != nil {
		t.Fatalf("save activity: %v", err)
	}

	// FindByID.
	got, err := repo.FindByID(ctx, a.ID)
	if err != nil || got == nil {
		t.Fatalf("find by id: %v, %+v", err, got)
	}
	if got.Duration != 45*time.Minute || got.AvgHeartRate != 150 {
		t.Fatalf("activity roundtrip mismatch: %+v", got)
	}

	// Dedup por external_id.
	byExt, err := repo.FindByExternalID(ctx, ext)
	if err != nil || byExt == nil {
		t.Fatalf("find by external id: %v, %+v", err, byExt)
	}
	if byExt.ID != a.ID {
		t.Fatalf("external id resolved to wrong activity: %s", byExt.ID)
	}

	// FindByUserID com range que cobre a atividade.
	list, err := repo.FindByUserID(ctx, userID, now.Add(-24*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("find by user: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 activity in range, got %d", len(list))
	}

	// Range que não cobre -> vazio.
	empty, _ := repo.FindByUserID(ctx, userID, now.Add(-72*time.Hour), now.Add(-48*time.Hour))
	if len(empty) != 0 {
		t.Fatalf("expected empty out-of-range result, got %d", len(empty))
	}
}

func TestIntegration_MetricRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	userID := uniqueID("user")
	mustSaveUser(t, db, userID)

	repo := postgres.NewMetricRepository(db)
	now := time.Now().UTC()

	// Insere 3 HRV em dias distintos.
	for i := 1; i <= 3; i++ {
		m, err := entity.NewMetric(userID, valueobject.MetricTypeHRV, float64(60+i), now.Add(-time.Duration(i)*24*time.Hour))
		if err != nil {
			t.Fatalf("new metric: %v", err)
		}
		if err := repo.Save(ctx, m); err != nil {
			t.Fatalf("save metric: %v", err)
		}
	}

	// Find by type + range.
	got, err := repo.FindByUserIDAndType(ctx, userID, valueobject.MetricTypeHRV, now.Add(-4*24*time.Hour), now)
	if err != nil {
		t.Fatalf("find by type: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 metrics, got %d", len(got))
	}
	// Ordenado ASC por data.
	if !got[0].Date.Before(got[1].Date) {
		t.Fatalf("expected ascending order by date")
	}

	// Latest: o mais recente primeiro (i=1, value=61).
	latest, err := repo.FindLatestByUserIDAndType(ctx, userID, valueobject.MetricTypeHRV, 1)
	if err != nil {
		t.Fatalf("find latest: %v", err)
	}
	if len(latest) != 1 || latest[0].Value != 61 {
		t.Fatalf("latest mismatch: %+v", latest)
	}

	// Outro tipo sem dados -> vazio.
	none, _ := repo.FindByUserIDAndType(ctx, userID, valueobject.MetricTypeWeight, now.Add(-4*24*time.Hour), now)
	if len(none) != 0 {
		t.Fatalf("expected no weight metrics, got %d", len(none))
	}
}

func TestIntegration_InsightRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	userID := uniqueID("user")
	mustSaveUser(t, db, userID)

	repo := postgres.NewInsightRepository(db)
	now := time.Now().UTC()

	i1, _ := entity.NewInsight(userID, valueobject.InsightTypeHRVDrop, valueobject.SeverityWarning, "hrv down", now.Add(-time.Hour))
	i2, _ := entity.NewInsight(userID, valueobject.InsightTypeOvertraining, valueobject.SeverityCritical, "acwr high", now.Add(-2*time.Hour))

	if err := repo.Save(ctx, i1); err != nil {
		t.Fatalf("save insight: %v", err)
	}
	if err := repo.SaveAll(ctx, []*entity.Insight{i2}); err != nil {
		t.Fatalf("save all insights: %v", err)
	}

	list, err := repo.FindByUserID(ctx, userID, now.Add(-24*time.Hour), now)
	if err != nil {
		t.Fatalf("list insights: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 insights, got %d", len(list))
	}
}

func TestIntegration_ProviderTokenRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	userID := uniqueID("user")
	mustSaveUser(t, db, userID)

	repo := postgres.NewProviderTokenRepository(db)
	athlete := uniqueID("athlete")

	tok := port.ProviderToken{
		Provider:     "strava",
		AccessToken:  "access-123",
		RefreshToken: "refresh-123",
		ExpiresAt:    time.Now().Add(time.Hour).UTC(),
		Scope:        "read,activity:read",
		AthleteID:    athlete,
	}
	if err := repo.Save(ctx, userID, tok); err != nil {
		t.Fatalf("save token: %v", err)
	}

	got, err := repo.Find(ctx, userID, "strava")
	if err != nil || got == nil {
		t.Fatalf("find token: %v, %+v", err, got)
	}
	if got.AccessToken != "access-123" || got.AthleteID != athlete {
		t.Fatalf("token roundtrip mismatch: %+v", got)
	}

	resolved, found, err := repo.FindUserByAthlete(ctx, "strava", athlete)
	if err != nil || !found {
		t.Fatalf("find user by athlete: found=%v err=%v", found, err)
	}
	if resolved != userID {
		t.Fatalf("resolved wrong user: %s", resolved)
	}

	// Athlete desconhecido -> found=false, sem erro.
	_, found2, err := repo.FindUserByAthlete(ctx, "strava", "unknown-athlete")
	if err != nil || found2 {
		t.Fatalf("expected not found for unknown athlete; found=%v err=%v", found2, err)
	}
}

func TestIntegration_DeviceRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	userID := uniqueID("user")
	mustSaveUser(t, db, userID)

	repo := postgres.NewDeviceRepository(db)
	token := uniqueID("devtoken")

	if err := repo.Save(ctx, userID, token, "android"); err != nil {
		t.Fatalf("save device: %v", err)
	}

	devices, err := repo.FindByUser(ctx, userID)
	if err != nil {
		t.Fatalf("find devices: %v", err)
	}
	if len(devices) != 1 || devices[0].Token != token || devices[0].Platform != "android" {
		t.Fatalf("device roundtrip mismatch: %+v", devices)
	}

	// Upsert por token: muda a plataforma sem duplicar.
	if err := repo.Save(ctx, userID, token, "ios"); err != nil {
		t.Fatalf("upsert device: %v", err)
	}
	devices, _ = repo.FindByUser(ctx, userID)
	if len(devices) != 1 || devices[0].Platform != "ios" {
		t.Fatalf("upsert did not update device: %+v", devices)
	}
}

func TestIntegration_NotificationRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	userID := uniqueID("user")
	mustSaveUser(t, db, userID)

	repo := postgres.NewNotificationRepository(db)

	rec := port.NotificationRecord{
		UserID:    userID,
		InsightID: uniqueID("insight"),
		Type:      "recovery_needed",
		Severity:  "critical",
		Title:     "Recovery needed",
		Body:      "Take a rest day",
		Status:    port.NotificationStatusSent,
	}
	if err := repo.Save(ctx, rec); err != nil {
		t.Fatalf("save notification: %v", err)
	}

	list, err := repo.ListByUser(ctx, userID, 10)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	if len(list) != 1 || list[0].Status != port.NotificationStatusSent || list[0].Title != "Recovery needed" {
		t.Fatalf("notification roundtrip mismatch: %+v", list)
	}
	if list[0].ID == "" {
		t.Fatal("expected generated notification ID")
	}
}

func TestIntegration_AggregatedMetricRepository(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	userID := uniqueID("user")
	mustSaveUser(t, db, userID)

	repo := postgres.NewAggregatedMetricRepository(db)
	day := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)

	agg := &port.DailyMetricAggregate{
		UserID:     userID,
		Date:       day,
		MetricType: "hrv",
		Count:      3,
		Sum:        180,
		Avg:        60,
		Min:        50,
		Max:        70,
	}
	if err := repo.Upsert(ctx, agg); err != nil {
		t.Fatalf("upsert aggregate: %v", err)
	}

	got, err := repo.Find(ctx, userID, day, "hrv")
	if err != nil || got == nil {
		t.Fatalf("find aggregate: %v, %+v", err, got)
	}
	if got.Count != 3 || got.Avg != 60 {
		t.Fatalf("aggregate roundtrip mismatch: %+v", got)
	}

	// Upsert idempotente do mesmo dia: sobrescreve, não duplica.
	agg.Count = 5
	agg.Avg = 62
	if err := repo.Upsert(ctx, agg); err != nil {
		t.Fatalf("re-upsert aggregate: %v", err)
	}
	got, _ = repo.Find(ctx, userID, day, "hrv")
	if got.Count != 5 || got.Avg != 62 {
		t.Fatalf("upsert did not update aggregate: %+v", got)
	}
}

// mustSaveUser cria um usuário mínimo para satisfazer as FKs das demais tabelas.
func mustSaveUser(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	repo := postgres.NewUserRepository(db)
	u := &entity.User{
		ID:           id,
		Name:         "FK User",
		Email:        id + "@example.com",
		PasswordHash: "$argon2id$v=19$m=1,t=1,p=1$abc$def",
		BirthDate:    time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		CreatedAt:    time.Now().UTC(),
	}
	if err := repo.Save(context.Background(), u); err != nil {
		t.Fatalf("seed FK user: %v", err)
	}
}
