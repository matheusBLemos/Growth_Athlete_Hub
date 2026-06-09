package main

import (
	"context"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

type noopEventPublisher struct{}

func (p *noopEventPublisher) Publish(_ context.Context, _ port.Event) error {
	return nil
}
