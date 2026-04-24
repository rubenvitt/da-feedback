package ui_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type adminTestEnv struct {
	mux      *http.ServeMux
	sessions *auth.SessionStore
	evenings *evening.Store
	group    *group.Group
	evening  *evening.Evening
}

func setupAdminTest(t *testing.T) adminTestEnv {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	groups := group.NewStore(db)
	evenings := evening.NewStore(db)
	surveys := survey.NewStore(db)
	analysisStore := analysis.NewStore(db, surveys)
	sessions := auth.NewSessionStore(db, 7*24*time.Hour)
	renderer, err := ui.NewRenderer(os.DirFS("../../templates"))
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	ctx := context.Background()
	grp, err := groups.Create(ctx, "I&K", "iuk")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	eve, err := evenings.Create(ctx, grp.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("create evening: %v", err)
	}

	mux := ui.NewRouter(ui.RouterConfig{
		BaseURL:  "http://example.test",
		Groups:   groups,
		Evenings: evenings,
		Surveys:  surveys,
		Analysis: analysisStore,
		Sessions: sessions,
		Renderer: renderer,
	})

	return adminTestEnv{
		mux:      mux,
		sessions: sessions,
		evenings: evenings,
		group:    grp,
		evening:  eve,
	}
}

func createSession(t *testing.T, sessions *auth.SessionStore, role string) *auth.Session {
	t.Helper()

	sess, err := sessions.Create(context.Background(), &auth.User{
		ID:    "test-" + role,
		Name:  "Test " + role,
		Email: role + "@example.test",
		Role:  role,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	return sess
}

func TestAdminCanDeleteEvening(t *testing.T) {
	env := setupAdminTest(t)
	sess := createSession(t, env.sessions, "admin")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/da/%d/delete", env.evening.ID), nil)
	req.AddCookie(&http.Cookie{Name: "daf_session", Value: sess.ID})
	rec := httptest.NewRecorder()

	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rec.Code)
	}
	if got, want := rec.Header().Get("Location"), fmt.Sprintf("/admin/groups/%d", env.group.ID); got != want {
		t.Fatalf("expected redirect %q, got %q", want, got)
	}
	if _, err := env.evenings.GetByID(context.Background(), env.evening.ID); err == nil {
		t.Fatal("expected evening to be deleted")
	}
}

func TestGroupLeaderCannotDeleteEvening(t *testing.T) {
	env := setupAdminTest(t)
	sess := createSession(t, env.sessions, "groupleader")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/da/%d/delete", env.evening.ID), nil)
	req.AddCookie(&http.Cookie{Name: "daf_session", Value: sess.ID})
	rec := httptest.NewRecorder()

	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
	if _, err := env.evenings.GetByID(context.Background(), env.evening.ID); err != nil {
		t.Fatalf("expected evening to remain, got error: %v", err)
	}
}

func TestAdminGroupDetailShowsEveningDeleteAction(t *testing.T) {
	env := setupAdminTest(t)
	sess := createSession(t, env.sessions, "admin")

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/admin/groups/%d", env.group.ID), nil)
	req.AddCookie(&http.Cookie{Name: "daf_session", Value: sess.ID})
	rec := httptest.NewRecorder()

	env.mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	wantAction := fmt.Sprintf(`action="/admin/da/%d/delete"`, env.evening.ID)
	if !strings.Contains(body, wantAction) {
		t.Fatalf("expected delete form action %q in group detail", wantAction)
	}
	if !strings.Contains(body, "Dienstabend löschen") {
		t.Fatal("expected delete button text")
	}
}
