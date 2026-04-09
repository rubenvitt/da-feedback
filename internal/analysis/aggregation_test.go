package analysis_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

type testEnv struct {
	analysis *analysis.Store
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
		analysis: analysis.NewStore(db),
		surveys:  survey.NewStore(db),
		evenings: evening.NewStore(db),
		groups:   group.NewStore(db),
	}
}

func TestGetDAStats(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()

	g, _ := env.groups.Create(ctx, "I&K", "iuk")
	topic := "Funk"
	e, _ := env.evenings.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), &topic)
	s, _ := env.surveys.Create(ctx, e.ID, nil)
	env.surveys.Activate(ctx, s.ID, 48)

	env.surveys.SubmitResponse(ctx, s.ID, map[string]any{
		"q1": 5, "q2": 4, "q3": 3, "q4": "Toll!", "q5": "", "q6": "HF-Funk",
	})
	env.surveys.SubmitResponse(ctx, s.ID, map[string]any{
		"q1": 3, "q2": 4, "q3": 5, "q4": "", "q5": "Mehr Praxis", "q6": "",
	})

	stats, err := env.analysis.GetDAStats(ctx, e.ID)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.ResponseCount != 2 {
		t.Fatalf("expected 2 responses, got %d", stats.ResponseCount)
	}
	if stats.AvgOverall != 4.0 {
		t.Fatalf("expected avg 4.0, got %.2f", stats.AvgOverall)
	}
	if len(stats.Highlights) != 1 || stats.Highlights[0] != "Toll!" {
		t.Fatalf("unexpected highlights: %v", stats.Highlights)
	}
	if len(stats.TopicWishes) != 1 || stats.TopicWishes[0] != "HF-Funk" {
		t.Fatalf("unexpected topic wishes: %v", stats.TopicWishes)
	}
}

func TestGetGroupComparisons(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()

	g1, _ := env.groups.Create(ctx, "I&K", "iuk")
	g2, _ := env.groups.Create(ctx, "San", "san")

	e1, _ := env.evenings.Create(ctx, g1.ID, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), nil)
	e2, _ := env.evenings.Create(ctx, g2.ID, time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), nil)

	s1, _ := env.surveys.Create(ctx, e1.ID, nil)
	env.surveys.Activate(ctx, s1.ID, 48)
	env.surveys.SubmitResponse(ctx, s1.ID, map[string]any{"q1": 5, "q2": 5, "q3": 5})

	s2, _ := env.surveys.Create(ctx, e2.ID, nil)
	env.surveys.Activate(ctx, s2.ID, 48)
	env.surveys.SubmitResponse(ctx, s2.ID, map[string]any{"q1": 3, "q2": 3, "q3": 3})

	comps, err := env.analysis.GetGroupComparisons(ctx)
	if err != nil {
		t.Fatalf("get comparisons: %v", err)
	}
	if len(comps) != 2 {
		t.Fatalf("expected 2, got %d", len(comps))
	}
}
