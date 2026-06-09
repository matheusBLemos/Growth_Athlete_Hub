package port

import "context"

type Event struct {
	Type    string
	Payload any
}

type EventPublisher interface {
	Publish(ctx context.Context, event Event) error
}
