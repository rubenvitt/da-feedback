package survey_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

type testEnv struct {
	surveys  *survey.Store
	evenings *evening.Store
	groups   *group.Store
}

func setupTestDB(t *testing.T) testEnv {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return testEnv{
		surveys:  survey.NewStore(db),
		evenings: evening.NewStore(db),
		groups:   group.NewStore(db),
	}
}

func createTestEvening(t *testing.T, env testEnv) (*group.Group, *evening.Evening) {
	t.Helper()
	ctx := context.Background()
	g, err := env.groups.Create(ctx, "I&K", "iuk")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	e, err := env.evenings.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("create evening: %v", err)
	}
	return g, e
}

func TestCreateSurvey(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, err := env.surveys.Create(ctx, e.ID, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Status != survey.StatusDraft {
		t.Fatalf("expected draft, got %s", s.Status)
	}
	if len(s.Questions) != 6 {
		t.Fatalf("expected 6 standard questions, got %d", len(s.Questions))
	}
}

func TestActivateSurvey(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	err := env.surveys.Activate(ctx, s.ID, 48)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}

	got, _ := env.surveys.GetByID(ctx, s.ID)
	if got.Status != survey.StatusActive {
		t.Fatalf("expected active, got %s", got.Status)
	}
	if got.ActivatedAt == nil || got.ClosesAt == nil {
		t.Fatal("expected activated_at and closes_at to be set")
	}
}

func TestOnlyOneActiveSurveyPerGroup(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	g, e1 := createTestEvening(t, env)

	e2, _ := env.evenings.Create(ctx, g.ID, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil)

	s1, _ := env.surveys.Create(ctx, e1.ID, nil)
	env.surveys.Activate(ctx, s1.ID, 48)

	s2, _ := env.surveys.Create(ctx, e2.ID, nil)
	env.surveys.Activate(ctx, s2.ID, 48)

	// s1 sollte jetzt geschlossen sein
	got, _ := env.surveys.GetByID(ctx, s1.ID)
	if got.Status != survey.StatusClosed {
		t.Fatalf("expected s1 closed, got %s", got.Status)
	}
}

func TestGetActiveSurveyForGroup(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	g, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	env.surveys.Activate(ctx, s.ID, 48)

	active, err := env.surveys.GetActiveForGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active.ID != s.ID {
		t.Fatalf("expected %d, got %d", s.ID, active.ID)
	}
}

func TestSubmitResponse(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	env.surveys.Activate(ctx, s.ID, 48)

	answers := map[string]any{"q1": 5, "q2": 4, "q3": 3, "q4": "Gut!", "q5": "", "q6": ""}
	r, err := env.surveys.SubmitResponse(ctx, s.ID, answers)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if r.SurveyID != s.ID {
		t.Fatalf("expected survey %d, got %d", s.ID, r.SurveyID)
	}

	responses, err := env.surveys.GetResponses(ctx, s.ID)
	if err != nil {
		t.Fatalf("get responses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
}

func TestCloseSurvey(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	env.surveys.Activate(ctx, s.ID, 48)

	err := env.surveys.Close(ctx, s.ID)
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	got, _ := env.surveys.GetByID(ctx, s.ID)
	if got.Status != survey.StatusClosed {
		t.Fatalf("expected closed, got %s", got.Status)
	}
	if got.ClosedAt == nil {
		t.Fatal("expected closed_at to be set")
	}
}
