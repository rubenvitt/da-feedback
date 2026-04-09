package ui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
	"github.com/rubeen/da-feedback/internal/ui"
)

func setupPublicTest(t *testing.T) (*http.ServeMux, *group.Group, *survey.Survey) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(db)
	t.Cleanup(func() { db.Close() })

	gs := group.NewStore(db)
	es := evening.NewStore(db)
	ss := survey.NewStore(db)

	ctx := context.Background()
	g, _ := gs.Create(ctx, "I&K", "iuk")
	e, _ := es.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	s, _ := ss.Create(ctx, e.ID, nil)
	ss.Activate(ctx, s.ID, 48)

	mux := http.NewServeMux()
	return mux, g, s
}

func TestSubmitFlow(t *testing.T) {
	db, _ := database.Open(":memory:")
	database.Migrate(db)
	defer db.Close()

	gs := group.NewStore(db)
	es := evening.NewStore(db)
	ss := survey.NewStore(db)

	ctx := context.Background()
	g, _ := gs.Create(ctx, "I&K", "iuk")
	e, _ := es.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	s, _ := ss.Create(ctx, e.ID, nil)
	ss.Activate(ctx, s.ID, 48)

	answers := map[string]any{
		"q1": 2, "q2": 1, "q3": 3, "q4": 2, "q5": 1, "q6": 2, "q7": 3, "q8": 1,
	}
	_, err := ss.SubmitResponse(ctx, s.ID, answers)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	count, _ := ss.GetResponseCount(ctx, s.ID)
	if count != 1 {
		t.Fatalf("expected 1 response, got %d", count)
	}

	_ = g
	_ = url.Values{}
	_ = strings.NewReader("")
	_ = httptest.NewRequest
	_ = httptest.NewRecorder
	_ = ui.NewPublicHandler
}
