package ui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/auth"
	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
	"github.com/rubeen/da-feedback/internal/ui"
)

type publicTestEnv struct {
	mux    *http.ServeMux
	group  *group.Group
	survey *survey.Survey
}

func setupPublicTest(t *testing.T) publicTestEnv {
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
	analysisStore := analysis.NewStore(db, ss)
	sessions := auth.NewSessionStore(db, 7*24*time.Hour)
	renderer, err := ui.NewRenderer(os.DirFS("../../templates"))
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	ctx := context.Background()
	g, _ := gs.Create(ctx, "I&K", "iuk")
	e, _ := es.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	s, _ := ss.Create(ctx, e.ID, nil)
	ss.Activate(ctx, s.ID, 48)

	mux := http.NewServeMux()
	mux = ui.NewRouter(ui.RouterConfig{
		BaseURL:  "http://example.test",
		Groups:   gs,
		Evenings: es,
		Surveys:  ss,
		Analysis: analysisStore,
		Sessions: sessions,
		Renderer: renderer,
	})

	return publicTestEnv{mux: mux, group: g, survey: s}
}

func TestSurveyExplainsSchoolGradesClearly(t *testing.T) {
	env := setupPublicTest(t)

	req := httptest.NewRequest(http.MethodGet, "/f/"+env.group.Slug+"-"+env.group.Secret, nil)
	rec := httptest.NewRecorder()

	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Schulnoten bewerten",
		"1 = sehr gut",
		"6 = schlecht",
		"Sehr gut",
		"Schlecht",
		"var(--survey-btn-border, #e5e7eb)",
		"var(--survey-input-border, #e5e7eb)",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected survey page to contain %q", want)
		}
	}
	for _, unwanted := range []string{
		"school-grade-guide",
		"Note 6 ist schlecht",
	} {
		if strings.Contains(body, unwanted) {
			t.Fatalf("expected survey page not to contain %q", unwanted)
		}
	}
	firstSix := strings.Index(body, `data-value="6"`)
	firstOne := strings.Index(body, `data-value="1"`)
	if firstSix == -1 || firstOne == -1 {
		t.Fatal("expected survey page to contain grade buttons for 6 and 1")
	}
	if firstSix > firstOne {
		t.Fatal("expected grade 6 to render before grade 1 so grade 1 appears on the right")
	}
	if strings.Index(body, `matrix-legend-left">Schlecht`) > strings.Index(body, `matrix-legend-right">Sehr gut`) {
		t.Fatal("expected legend to show Schlecht on the left and Sehr gut on the right")
	}
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
