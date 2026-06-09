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

var (
	_ port.DeviceRepository = (*fakeDeviceRepo)(nil)
	_ port.Notifier         = (*fakeNotifier)(nil)
)
