package notifications

import (
	"context"
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

func TestLogNotifier_Send_AlwaysSucceeds(t *testing.T) {
	n := NewLogNotifier()
	err := n.Send(context.Background(), port.Notification{
		UserID: "u1",
		Token:  "tok-1",
		Title:  "Hello",
		Body:   "World",
		Data:   map[string]string{"insight_id": "ins-1"},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
}
