package survey_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/survey"
)

func TestCloseExpired(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	// Aktivieren mit 0 Stunden → sofort abgelaufen
	env.surveys.Activate(ctx, s.ID, 0)

	// Kurz warten damit closes_at in der Vergangenheit liegt
	time.Sleep(10 * time.Millisecond)

	n, err := env.surveys.CloseExpired(ctx)
	if err != nil {
		t.Fatalf("close expired: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 closed, got %d", n)
	}

	got, _ := env.surveys.GetByID(ctx, s.ID)
	if got.Status != survey.StatusClosed {
		t.Fatalf("expected closed, got %s", got.Status)
	}
}

func TestAutoCloserStartStop(t *testing.T) {
	env := setupTestDB(t)
	closer := survey.NewAutoCloser(env.surveys, 50*time.Millisecond)
	closer.Start()
	time.Sleep(100 * time.Millisecond)
	closer.Stop()
	// Kein Panic, kein Hang → Test bestanden
}
