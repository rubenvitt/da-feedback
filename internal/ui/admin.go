package ui

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/auth"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/qrcode"
	"github.com/rubeen/da-feedback/internal/survey"
)

type AdminHandler struct {
	groups   *group.Store
	evenings *evening.Store
	surveys  *survey.Store
	analysis *analysis.Store
	sessions *auth.SessionStore
	render   *Renderer
	baseURL  string
}

func NewAdminHandler(g *group.Store, e *evening.Store, s *survey.Store, a *analysis.Store, sess *auth.SessionStore, r *Renderer, baseURL string) *AdminHandler {
	return &AdminHandler{groups: g, evenings: e, surveys: s, analysis: a, sessions: sess, render: r, baseURL: baseURL}
}

func (h *AdminHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(fn http.HandlerFunc) http.Handler { return authMw(fn) }

	mux.Handle("GET /admin", wrap(h.dashboard))
	mux.Handle("GET /admin/groups", wrap(h.listGroups))
	mux.Handle("POST /admin/groups", wrap(h.createGroup))
	mux.Handle("GET /admin/groups/{id}", wrap(h.showGroup))
	mux.Handle("POST /admin/groups/{id}", wrap(h.updateGroup))
	mux.Handle("POST /admin/groups/{id}/delete", wrap(h.deleteGroup))
	mux.Handle("POST /admin/groups/{id}/regenerate-secret", wrap(h.regenerateSecret))
	mux.Handle("GET /admin/groups/{id}/qr.png", wrap(h.groupQRCode))
	mux.Handle("POST /admin/groups/{id}/da", wrap(h.createEvening))
	mux.Handle("GET /admin/da/{id}", wrap(h.showEvening))
	mux.Handle("POST /admin/da/{id}", wrap(h.updateEvening))
}

func (h *AdminHandler) dashboard(w http.ResponseWriter, r *http.Request) {
	sess := auth.GetSession(r)
	ctx := r.Context()

	var groups []group.Group
	if sess.User.Role == "admin" {
		groups, _ = h.groups.List(ctx)
	} else {
		ids, _ := h.sessions.GetUserGroups(ctx, sess.UserID)
		for _, id := range ids {
			if g, err := h.groups.GetByID(ctx, id); err == nil {
				groups = append(groups, *g)
			}
		}
	}

	h.render.Render(w, "admin/dashboard.html", http.StatusOK, map[string]any{
		"User":   sess.User,
		"Groups": groups,
	})
}

func (h *AdminHandler) listGroups(w http.ResponseWriter, r *http.Request) {
	groups, _ := h.groups.List(r.Context())
	h.render.Render(w, "admin/groups.html", http.StatusOK, map[string]any{
		"User":   auth.GetSession(r).User,
		"Groups": groups,
	})
}

func (h *AdminHandler) createGroup(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	name := r.FormValue("name")
	slug := r.FormValue("slug")
	if name == "" || slug == "" {
		http.Error(w, "Name und Slug erforderlich", http.StatusBadRequest)
		return
	}
	h.groups.Create(r.Context(), name, slug)
	http.Redirect(w, r, "/admin/groups", http.StatusSeeOther)
}

func (h *AdminHandler) showGroup(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()

	grp, err := h.groups.GetByID(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	evenings, _ := h.evenings.ListByGroup(ctx, id)

	type EveningWithSurvey struct {
		Evening       evening.Evening
		Survey        *survey.Survey
		ResponseCount int
	}
	var items []EveningWithSurvey
	for _, e := range evenings {
		s, _ := h.surveys.GetByEveningID(ctx, e.ID)
		var rc int
		if s != nil {
			rc, _ = h.surveys.GetResponseCount(ctx, s.ID)
		}
		items = append(items, EveningWithSurvey{Evening: e, Survey: s, ResponseCount: rc})
	}

	feedbackURL := qrcode.FeedbackURL(h.baseURL, grp.Slug, grp.Secret)

	h.render.Render(w, "admin/group_detail.html", http.StatusOK, map[string]any{
		"User":        auth.GetSession(r).User,
		"Group":       grp,
		"Items":       items,
		"FeedbackURL": feedbackURL,
	})
}

func (h *AdminHandler) updateGroup(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()
	grp, err := h.groups.GetByID(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	grp.Name = r.FormValue("name")
	if v := r.FormValue("close_after_hours"); v != "" {
		n, _ := strconv.Atoi(v)
		grp.CloseAfterHours = &n
	}
	h.groups.Update(ctx, grp)
	http.Redirect(w, r, fmt.Sprintf("/admin/groups/%d", id), http.StatusSeeOther)
}

func (h *AdminHandler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	h.groups.Delete(r.Context(), id)
	http.Redirect(w, r, "/admin/groups", http.StatusSeeOther)
}

func (h *AdminHandler) regenerateSecret(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	h.groups.RegenerateSecret(r.Context(), id)
	http.Redirect(w, r, fmt.Sprintf("/admin/groups/%d", id), http.StatusSeeOther)
}

func (h *AdminHandler) groupQRCode(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	grp, err := h.groups.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	png, err := qrcode.GeneratePNG(h.baseURL, grp.Slug, grp.Secret, 300)
	if err != nil {
		http.Error(w, "QR-Code Fehler", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="qr-%s.png"`, grp.Slug))
	w.Write(png)
}

func (h *AdminHandler) createEvening(w http.ResponseWriter, r *http.Request) {
	groupID, _ := strconv.Atoi(r.PathValue("id"))
	r.ParseForm()

	date, err := time.Parse("2006-01-02", r.FormValue("date"))
	if err != nil {
		http.Error(w, "Ungültiges Datum", http.StatusBadRequest)
		return
	}

	var topic *string
	if t := r.FormValue("topic"); t != "" {
		topic = &t
	}

	h.evenings.Create(r.Context(), groupID, date, topic)
	http.Redirect(w, r, fmt.Sprintf("/admin/groups/%d", groupID), http.StatusSeeOther)
}

func (h *AdminHandler) showEvening(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	eve, err := h.evenings.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.render.Render(w, "admin/evening_form.html", http.StatusOK, map[string]any{
		"User":    auth.GetSession(r).User,
		"Evening": eve,
	})
}

func (h *AdminHandler) updateEvening(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()
	eve, err := h.evenings.GetByID(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	r.ParseForm()
	if v := r.FormValue("participant_count"); v != "" {
		n, _ := strconv.Atoi(v)
		eve.ParticipantCount = &n
	}
	if t := r.FormValue("topic"); t != "" {
		eve.Topic = &t
	}
	if n := r.FormValue("notes"); n != "" {
		eve.Notes = &n
	}
	h.evenings.Update(ctx, eve)
	http.Redirect(w, r, fmt.Sprintf("/admin/groups/%d", eve.GroupID), http.StatusSeeOther)
}
