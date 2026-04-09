package ui

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/auth"
	"github.com/rubeen/da-feedback/internal/group"
)

type AnalysisHandler struct {
	analysis *analysis.Store
	groups   *group.Store
	render   *Renderer
}

func NewAnalysisHandler(a *analysis.Store, g *group.Store, r *Renderer) *AnalysisHandler {
	return &AnalysisHandler{analysis: a, groups: g, render: r}
}

func (h *AnalysisHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler, adminMw func(http.Handler) http.Handler) {
	wrap := func(fn http.HandlerFunc) http.Handler { return authMw(fn) }
	adminWrap := func(fn http.HandlerFunc) http.Handler { return authMw(adminMw(fn)) }

	mux.Handle("GET /admin/analysis/da/{id}", wrap(h.daAnalysis))
	mux.Handle("GET /admin/analysis/group/{id}", wrap(h.groupAnalysis))
	mux.Handle("GET /admin/analysis/global", adminWrap(h.globalAnalysis))
	mux.Handle("GET /admin/analysis/export/{id}", wrap(h.exportCSV))
}

func (h *AnalysisHandler) daAnalysis(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	stats, err := h.analysis.GetDAStats(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.render.Render(w, "admin/analysis_da.html", http.StatusOK, map[string]any{
		"User":  auth.GetSession(r).User,
		"Stats": stats,
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
