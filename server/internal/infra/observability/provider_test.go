package observability

import (
	"context"
	"testing"
)

// TestSetup_Disabled garante o comportamento disabled-safe: sem endpoint/flag,
// Setup não toca a rede, não falha e devolve um shutdown no-op.
func TestSetup_Disabled(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{Enabled: false})
	if err != nil {
		t.Fatalf("Setup(disabled) error = %v, want nil", err)
	}
	if shutdown == nil {
		t.Fatal("Setup must always return a non-nil shutdown func")
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("no-op shutdown error = %v, want nil", err)
	}
}

// TestSetup_EnabledWithoutEndpoint também é no-op (não há para onde exportar).
func TestSetup_EnabledWithoutEndpoint(t *testing.T) {
	shutdown, err := Setup(context.Background(), Config{Enabled: true, OTLPEndpoint: ""})
	if err != nil {
		t.Fatalf("Setup(no endpoint) error = %v, want nil", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown error = %v, want nil", err)
	}
}

func TestServiceName(t *testing.T) {
	if got := ServiceName("", "gah-api"); got != "gah-api" {
		t.Errorf("ServiceName(empty) = %q, want gah-api", got)
	}
	if got := ServiceName("custom", "gah-api"); got != "custom" {
		t.Errorf("ServiceName(custom) = %q, want custom", got)
	}
}
