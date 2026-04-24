package evening_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

func setupTestDB(t *testing.T) (*evening.Store, *group.Store) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return evening.NewStore(db), group.NewStore(db)
}

func TestCreateAndGetEvening(t *testing.T) {
	eStore, gStore := setupTestDB(t)
	ctx := context.Background()

	g, _ := gStore.Create(ctx, "I&K", "iuk")
	topic := "Funkausbildung"
	date := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)

	e, err := eStore.Create(ctx, g.ID, date, &topic)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.GroupID != g.ID || *e.Topic != "Funkausbildung" {
		t.Fatalf("unexpected evening: %+v", e)
	}

	got, err := eStore.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != e.ID {
		t.Fatalf("expected %d, got %d", e.ID, got.ID)
	}
}

func TestListByGroup(t *testing.T) {
	eStore, gStore := setupTestDB(t)
	ctx := context.Background()

	g, _ := gStore.Create(ctx, "I&K", "iuk")
	d1 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	eStore.Create(ctx, g.ID, d1, nil)
	eStore.Create(ctx, g.ID, d2, nil)

	evenings, err := eStore.ListByGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(evenings) != 2 {
		t.Fatalf("expected 2, got %d", len(evenings))
	}
	// Neueste zuerst
	if evenings[0].Date.Before(evenings[1].Date) {
		t.Fatal("expected newest first")
	}
}

func TestUpdateParticipantCount(t *testing.T) {
	eStore, gStore := setupTestDB(t)
	ctx := context.Background()

	g, _ := gStore.Create(ctx, "I&K", "iuk")
	date := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	e, _ := eStore.Create(ctx, g.ID, date, nil)

	count := 15
	err := eStore.UpdateParticipantCount(ctx, e.ID, &count)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := eStore.GetByID(ctx, e.ID)
	if got.ParticipantCount == nil || *got.ParticipantCount != 15 {
		t.Fatalf("expected 15, got %v", got.ParticipantCount)
	}
}

func TestDeleteEveningDeletesSurveyAndResponses(t *testing.T) {
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	eStore := evening.NewStore(db)
	gStore := group.NewStore(db)
	sStore := survey.NewStore(db)
	ctx := context.Background()

	g, _ := gStore.Create(ctx, "I&K", "iuk")
	e, _ := eStore.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	s, _ := sStore.Create(ctx, e.ID, nil)
	_, _ = sStore.SubmitResponse(ctx, s.ID, map[string]any{"q1": 1})

	if err := eStore.Delete(ctx, e.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := eStore.GetByID(ctx, e.ID); err == nil {
		t.Fatal("expected error after delete")
	}
	if _, err := sStore.GetByID(ctx, s.ID); err == nil {
		t.Fatal("expected survey to be deleted with evening")
	}
	count, err := sStore.GetResponseCount(ctx, s.ID)
	if err != nil {
		t.Fatalf("count responses: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 responses after delete, got %d", count)
	}
}
