package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
)

// fakeDeviceRepo é um repositório de dispositivos em memória para os testes.
type fakeDeviceRepo struct {
	byUser map[string][]port.Device
	err    error
}

func (r *fakeDeviceRepo) Save(_ context.Context, userID, token, platform string) error {
	if r.err != nil {
		return r.err
	}
	if r.byUser == nil {
		r.byUser = make(map[string][]port.Device)
	}
	r.byUser[userID] = append(r.byUser[userID], port.Device{UserID: userID, Token: token, Platform: platform})
	return nil
}

func (r *fakeDeviceRepo) FindByUser(_ context.Context, userID string) ([]port.Device, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.byUser[userID], nil
}

// fakeNotifier captura as notificações enviadas e pode falhar para tokens
// específicos, simulando falha por dispositivo.
type fakeNotifier struct {
	sent     []port.Notification
	failFor  map[string]error
	alwaysFn func(port.Notification) error
}

func (n *fakeNotifier) Send(_ context.Context, notif port.Notification) error {
	n.sent = append(n.sent, notif)
	if n.alwaysFn != nil {
		return n.alwaysFn(notif)
	}
	if err, ok := n.failFor[notif.Token]; ok {
		return err
	}
	return nil
}

// fakeNotificationRepo captura os registros de histórico persistidos.
type fakeNotificationRepo struct {
	saved   []port.NotificationRecord
	saveErr error
}

func (r *fakeNotificationRepo) Save(_ context.Context, record port.NotificationRecord) error {
	if r.saveErr != nil {
		return r.saveErr
	}
	r.saved = append(r.saved, record)
	return nil
}

func (r *fakeNotificationRepo) ListByUser(_ context.Context, userID string, limit int) ([]port.NotificationRecord, error) {
	if r.saveErr != nil {
		return nil, r.saveErr
	}
	var out []port.NotificationRecord
	for _, rec := range r.saved {
		if rec.UserID == userID {
			out = append(out, rec)
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func sampleInsight() usecase.InsightGenerated {
	return usecase.InsightGenerated{
		UserID:    "u1",
		InsightID: "ins-1",
		Type:      "recovery",
		Severity:  "warning",
		Message:   "Sua recuperação está baixa hoje.",
		Date:      time.Date(2026, 6, 9, 8, 0, 0, 0, time.UTC),
	}
}

func TestNotifyInsight_SendsToEachDevice(t *testing.T) {
	repo := &fakeDeviceRepo{byUser: map[string][]port.Device{
		"u1": {
			{UserID: "u1", Token: "tok-a", Platform: "android"},
			{UserID: "u1", Token: "tok-b", Platform: "ios"},
		},
	}}
	notifier := &fakeNotifier{}
	uc := usecase.NewNotifyInsight(repo, notifier)

	if err := uc.Execute(context.Background(), sampleInsight()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(notifier.sent) != 2 {
		t.Fatalf("sent %d notifications, want 2", len(notifier.sent))
	}
	// Conteúdo derivado do insight e dados estruturados presentes.
	for _, n := range notifier.sent {
		if n.Body != "Sua recuperação está baixa hoje." {
			t.Errorf("body = %q, want insight message", n.Body)
		}
		if n.Title == "" {
			t.Error("title should be derived and non-empty")
		}
		if n.UserID != "u1" {
			t.Errorf("user_id = %q, want u1", n.UserID)
		}
		if n.Data["insight_id"] != "ins-1" {
			t.Errorf("data[insight_id] = %q, want ins-1", n.Data["insight_id"])
		}
	}
}

func TestNotifyInsight_NoDevices_IsNoopSuccess(t *testing.T) {
	repo := &fakeDeviceRepo{byUser: map[string][]port.Device{}}
	notifier := &fakeNotifier{}
	uc := usecase.NewNotifyInsight(repo, notifier)

	if err := uc.Execute(context.Background(), sampleInsight()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(notifier.sent) != 0 {
		t.Fatalf("sent %d notifications, want 0", len(notifier.sent))
	}
}

// TestNotifyInsight_SingleDeviceFailure_ContinuesOthers garante que a falha em
// um dispositivo não aborta os demais: os outros recebem a notificação e a use
// case retorna erro (para permitir redelivery controlado).
func TestNotifyInsight_SingleDeviceFailure_ContinuesOthers(t *testing.T) {
	repo := &fakeDeviceRepo{byUser: map[string][]port.Device{
		"u1": {
			{UserID: "u1", Token: "tok-a"},
			{UserID: "u1", Token: "tok-bad"},
			{UserID: "u1", Token: "tok-c"},
		},
	}}
	notifier := &fakeNotifier{failFor: map[string]error{"tok-bad": errors.New("push rejected")}}
	uc := usecase.NewNotifyInsight(repo, notifier)

	err := uc.Execute(context.Background(), sampleInsight())
	if err == nil {
		t.Fatal("expected aggregated error when a device send fails")
	}
	// Todos os 3 dispositivos foram tentados (não abortou no primeiro erro).
	if len(notifier.sent) != 3 {
		t.Fatalf("sent %d notifications, want 3 (all attempted)", len(notifier.sent))
	}
}

func TestNotifyInsight_RepoError_Propagates(t *testing.T) {
	repo := &fakeDeviceRepo{err: errors.New("db down")}
	notifier := &fakeNotifier{}
	uc := usecase.NewNotifyInsight(repo, notifier)

	if err := uc.Execute(context.Background(), sampleInsight()); err == nil {
		t.Fatal("expected repo error to propagate")
	}
}

// TestNotifyInsight_PersistsHistory_SentAndFailed garante que cada tentativa de
// envio gera um registro de histórico com o status correto (sent/failed).
func TestNotifyInsight_PersistsHistory_SentAndFailed(t *testing.T) {
	repo := &fakeDeviceRepo{byUser: map[string][]port.Device{
		"u1": {
			{UserID: "u1", Token: "tok-a"},
			{UserID: "u1", Token: "tok-bad"},
		},
	}}
	notifier := &fakeNotifier{failFor: map[string]error{"tok-bad": errors.New("push rejected")}}
	history := &fakeNotificationRepo{}
	uc := usecase.NewNotifyInsight(repo, notifier).WithHistory(history)

	err := uc.Execute(context.Background(), sampleInsight())
	if err == nil {
		t.Fatal("expected aggregated error when a device send fails")
	}

	if len(history.saved) != 2 {
		t.Fatalf("persisted %d history records, want 2", len(history.saved))
	}

	var sent, failed int
	for _, rec := range history.saved {
		if rec.UserID != "u1" || rec.InsightID != "ins-1" {
			t.Errorf("record = %+v, want user u1 insight ins-1", rec)
		}
		switch rec.Status {
		case port.NotificationStatusSent:
			sent++
			if rec.Error != "" {
				t.Errorf("sent record should have empty error, got %q", rec.Error)
			}
		case port.NotificationStatusFailed:
			failed++
			if rec.Error == "" {
				t.Error("failed record should carry an error message")
			}
		default:
			t.Errorf("unexpected status %q", rec.Status)
		}
	}
	if sent != 1 || failed != 1 {
		t.Errorf("sent=%d failed=%d, want 1 and 1", sent, failed)
	}
}

// TestNotifyInsight_HistoryWriteError_DoesNotAbortDelivery garante que uma falha
// ao persistir o histórico não interrompe nem altera o resultado da entrega.
func TestNotifyInsight_HistoryWriteError_DoesNotAbortDelivery(t *testing.T) {
	repo := &fakeDeviceRepo{byUser: map[string][]port.Device{
		"u1": {{UserID: "u1", Token: "tok-a"}},
	}}
	notifier := &fakeNotifier{}
	history := &fakeNotificationRepo{saveErr: errors.New("history db down")}
	uc := usecase.NewNotifyInsight(repo, notifier).WithHistory(history)

	if err := uc.Execute(context.Background(), sampleInsight()); err != nil {
		t.Fatalf("history write error must not abort delivery: %v", err)
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("sent %d notifications, want 1", len(notifier.sent))
	}
}

// TestNotifyInsight_NilHistory_IsNoop garante que a use case opera sem histórico.
func TestNotifyInsight_NilHistory_IsNoop(t *testing.T) {
	repo := &fakeDeviceRepo{byUser: map[string][]port.Device{
		"u1": {{UserID: "u1", Token: "tok-a"}},
	}}
	notifier := &fakeNotifier{}
	uc := usecase.NewNotifyInsight(repo, notifier) // sem WithHistory

	if err := uc.Execute(context.Background(), sampleInsight()); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(notifier.sent) != 1 {
		t.Fatalf("sent %d notifications, want 1", len(notifier.sent))
	}
}

var (
	_ port.DeviceRepository       = (*fakeDeviceRepo)(nil)
	_ port.Notifier               = (*fakeNotifier)(nil)
	_ port.NotificationRepository = (*fakeNotificationRepo)(nil)
)
