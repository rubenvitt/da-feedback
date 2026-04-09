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
	ss := survey.NewStore(db)
	return testEnv{
		analysis: analysis.NewStore(db, ss),
		surveys:  ss,
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
		"q1": 1, "q2": 2, "q3": 1, "q4": 2, "q5": 1, "q6": 2, "q7": 1, "q8": 2,
		"q9": "Praxis war super!", "q11": "", "q13": "HF-Funk",
	})
	env.surveys.SubmitResponse(ctx, s.ID, map[string]any{
		"q1": 3, "q2": 2, "q3": 3, "q4": 2, "q5": 3, "q6": 2, "q7": 3, "q8": 2,
		"q9": "", "q11": "Mehr Praxis", "q13": "",
	})

	stats, err := env.analysis.GetDAStats(ctx, e.ID)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.ResponseCount != 2 {
		t.Fatalf("expected 2 responses, got %d", stats.ResponseCount)
	}
	if len(stats.Ratings) != 8 {
		t.Fatalf("expected 8 ratings, got %d", len(stats.Ratings))
	}
	// q1 avg: (1+3)/2 = 2.0
	if stats.Ratings[0].Avg != 2.0 {
		t.Fatalf("expected q1 avg 2.0, got %.2f", stats.Ratings[0].Avg)
	}
	// Overall avg: (1.5+2+2+2+2+2+2+2)/8 = 1.9375 ≈ 1.94
	if stats.AvgOverall < 1.9 || stats.AvgOverall > 2.1 {
		t.Fatalf("expected avg around 2.0, got %.2f", stats.AvgOverall)
	}

	// Text: q9 should have 1 response
	var foundHighlights bool
	for _, ta := range stats.TextAnswers {
		if ta.QuestionID == "q9" && len(ta.Responses) == 1 && ta.Responses[0] == "Praxis war super!" {
			foundHighlights = true
		}
	}
	if !foundHighlights {
		t.Fatal("q9 highlight not found")
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
	env.surveys.SubmitResponse(ctx, s1.ID, map[string]any{
		"q1": 1, "q2": 1, "q3": 1, "q4": 1, "q5": 1, "q6": 1, "q7": 1, "q8": 1,
	})

	s2, _ := env.surveys.Create(ctx, e2.ID, nil)
	env.surveys.Activate(ctx, s2.ID, 48)
	env.surveys.SubmitResponse(ctx, s2.ID, map[string]any{
		"q1": 3, "q2": 3, "q3": 3, "q4": 3, "q5": 3, "q6": 3, "q7": 3, "q8": 3,
	})

	comps, err := env.analysis.GetGroupComparisons(ctx)
	if err != nil {
		t.Fatalf("get comparisons: %v", err)
	}
	if len(comps) != 2 {
		t.Fatalf("expected 2, got %d", len(comps))
	}
}
