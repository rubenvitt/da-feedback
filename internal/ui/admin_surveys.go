package ui

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/rubeen/da-feedback/internal/auth"
	"github.com/rubeen/da-feedback/internal/survey"
)

func (h *AdminHandler) RegisterSurveyRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(fn http.HandlerFunc) http.Handler { return authMw(fn) }

	mux.Handle("POST /admin/da/{id}/survey", wrap(h.createSurvey))
	mux.Handle("GET /admin/surveys/{id}", wrap(h.showSurvey))
	mux.Handle("POST /admin/surveys/{id}/activate", wrap(h.activateSurvey))
	mux.Handle("POST /admin/surveys/{id}/close", wrap(h.closeSurvey))
	mux.Handle("POST /admin/surveys/{id}/archive", wrap(h.archiveSurvey))
}

func (h *AdminHandler) createSurvey(w http.ResponseWriter, r *http.Request) {
	eveningID, _ := strconv.Atoi(r.PathValue("id"))
	s, err := h.surveys.Create(r.Context(), eveningID, nil)
	if err != nil {
		http.Error(w, "Umfrage konnte nicht erstellt werden", http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/surveys/%d", s.ID), http.StatusSeeOther)
}

func (h *AdminHandler) showSurvey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()

	srv, err := h.surveys.GetByID(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	eve, _ := h.evenings.GetByID(ctx, srv.EveningID)
	grp, _ := h.groups.GetByID(ctx, eve.GroupID)
	count, _ := h.surveys.GetResponseCount(ctx, id)

	data := map[string]any{
		"User":          auth.GetSession(r).User,
		"Survey":        srv,
		"Evening":       eve,
		"Group":         grp,
		"ResponseCount": count,
	}

	if srv.Status == survey.StatusDraft {
		if active, err := h.surveys.GetActiveForGroup(ctx, eve.GroupID); err == nil && active != nil {
			data["ActiveSurveyExists"] = true
		}
	}

	if srv.Status == survey.StatusClosed || srv.Status == survey.StatusArchived {
		if stats, err := h.analysis.GetDAStats(ctx, srv.EveningID); err == nil {
			data["Stats"] = stats
		}
	}

	h.render.Render(w, "admin/survey_detail.html", http.StatusOK, data)
}

func (h *AdminHandler) activateSurvey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()

	srv, err := h.surveys.GetByID(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	closeHours := 48
	eve, _ := h.evenings.GetByID(ctx, srv.EveningID)
	grp, _ := h.groups.GetByID(ctx, eve.GroupID)

	if srv.CloseAfterHours != nil {
		closeHours = *srv.CloseAfterHours
	} else if grp.CloseAfterHours != nil {
		closeHours = *grp.CloseAfterHours
	}

	if err := h.surveys.Activate(ctx, id, closeHours); err != nil {
		http.Error(w, "Aktivierung fehlgeschlagen", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		srv, _ = h.surveys.GetByID(ctx, id)
		h.render.Render(w, "admin/survey_detail.html", http.StatusOK, map[string]any{
			"Survey": srv, "Evening": eve, "Group": grp,
		})
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/admin/surveys/%d", id), http.StatusSeeOther)
}

func (h *AdminHandler) closeSurvey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	h.surveys.Close(r.Context(), id)
	http.Redirect(w, r, fmt.Sprintf("/admin/surveys/%d", id), http.StatusSeeOther)
}

func (h *AdminHandler) archiveSurvey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	h.surveys.Archive(r.Context(), id)
	http.Redirect(w, r, fmt.Sprintf("/admin/surveys/%d", id), http.StatusSeeOther)
}
