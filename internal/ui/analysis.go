package ui

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/auth"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

type AnalysisHandler struct {
	analysis *analysis.Store
	groups   *group.Store
	evenings *evening.Store
	surveys  *survey.Store
	render   *Renderer
}

func NewAnalysisHandler(a *analysis.Store, g *group.Store, e *evening.Store, s *survey.Store, r *Renderer) *AnalysisHandler {
	return &AnalysisHandler{analysis: a, groups: g, evenings: e, surveys: s, render: r}
}

func (h *AnalysisHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler, adminMw func(http.Handler) http.Handler) {
	wrap := func(fn http.HandlerFunc) http.Handler { return authMw(fn) }
	adminWrap := func(fn http.HandlerFunc) http.Handler { return authMw(adminMw(fn)) }

	mux.Handle("GET /admin/analysis/da/{id}", wrap(h.daAnalysis))
	mux.Handle("GET /admin/analysis/group/{id}", wrap(h.groupAnalysis))
	mux.Handle("GET /admin/analysis/global", adminWrap(h.globalAnalysis))
	mux.Handle("GET /admin/analysis/export/{id}", wrap(h.exportCSV))
	mux.Handle("GET /admin/surveys/{id}/prompt", wrap(h.surveyPrompt))
}

func (h *AnalysisHandler) daAnalysis(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()
	stats, err := h.analysis.GetDAStats(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	var surveyID int
	var surveyStatus survey.Status
	if srv, err := h.surveys.GetByEveningID(ctx, id); err == nil {
		surveyID = srv.ID
		surveyStatus = srv.Status
	}

	h.render.Render(w, "admin/analysis_da.html", http.StatusOK, map[string]any{
		"User":         auth.GetSession(r).User,
		"Stats":        stats,
		"SurveyID":     surveyID,
		"SurveyStatus": surveyStatus,
	})
}

func (h *AnalysisHandler) groupAnalysis(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()

	grp, err := h.groups.GetByID(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	to := time.Now()
	from := to.AddDate(-1, 0, 0)
	if v := r.URL.Query().Get("from"); v != "" {
		from, _ = time.Parse("2006-01-02", v)
	}
	if v := r.URL.Query().Get("to"); v != "" {
		to, _ = time.Parse("2006-01-02", v)
	}

	trend, _ := h.analysis.GetGroupTrend(ctx, id, from, to)

	h.render.Render(w, "admin/analysis_group.html", http.StatusOK, map[string]any{
		"User":  auth.GetSession(r).User,
		"Group": grp,
		"Trend": trend,
		"From":  from.Format("2006-01-02"),
		"To":    to.Format("2006-01-02"),
	})
}

func (h *AnalysisHandler) globalAnalysis(w http.ResponseWriter, r *http.Request) {
	comps, _ := h.analysis.GetGroupComparisons(r.Context())
	h.render.Render(w, "admin/analysis_global.html", http.StatusOK, map[string]any{
		"User":        auth.GetSession(r).User,
		"Comparisons": comps,
	})
}

func (h *AnalysisHandler) exportCSV(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))

	grp, err := h.groups.GetByID(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	to := time.Now()
	from := to.AddDate(-1, 0, 0)

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\"export-"+grp.Slug+".csv\"")

	h.analysis.ExportGroupCSV(r.Context(), w, id, from, to)
}

func (h *AnalysisHandler) surveyPrompt(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()

	srv, err := h.surveys.GetByID(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if srv.Status == survey.StatusActive {
		http.Error(w, "Auswertung ist nicht verfügbar, solange die Umfrage noch läuft", http.StatusForbidden)
		return
	}

	eve, err := h.evenings.GetByID(ctx, srv.EveningID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	grp, err := h.groups.GetByID(ctx, eve.GroupID)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	stats, err := h.analysis.GetDAStats(ctx, srv.EveningID)
	if err != nil || stats.ResponseCount == 0 {
		http.Error(w, "Keine Daten verfügbar", http.StatusNotFound)
		return
	}

	responses, err := h.surveys.GetResponses(ctx, id)
	if err != nil {
		http.Error(w, "Antworten konnten nicht geladen werden", http.StatusInternalServerError)
		return
	}

	prompt := analysis.BuildPrompt(stats, responses, *grp, *eve)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write([]byte(prompt))
}
