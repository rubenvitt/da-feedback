package ui

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

type PublicHandler struct {
	groups   *group.Store
	evenings *evening.Store
	surveys  *survey.Store
	render   *Renderer
	baseURL  string
}

func NewPublicHandler(g *group.Store, e *evening.Store, s *survey.Store, r *Renderer, baseURL string) *PublicHandler {
	return &PublicHandler{groups: g, evenings: e, surveys: s, render: r, baseURL: baseURL}
}

func (h *PublicHandler) ShowSurvey(w http.ResponseWriter, r *http.Request) {
	slugSecret := r.PathValue("slugSecret")

	if strings.HasPrefix(slugSecret, "alle-") {
		h.showGroupSelect(w, r, strings.TrimPrefix(slugSecret, "alle-"))
		return
	}

	grp, srv, eve, err := h.resolve(r, slugSecret)
	if err != nil {
		h.render.Render(w, "public/unavailable.html", http.StatusOK, nil)
		return
	}

	alreadySubmitted := false
	if cookie, err := r.Cookie(fmt.Sprintf("feedback-%d", srv.ID)); err == nil && cookie.Value == "submitted" {
		alreadySubmitted = true
	}

	h.render.Render(w, "public/survey.html", http.StatusOK, map[string]any{
		"Group":            grp,
		"Evening":          eve,
		"Survey":           srv,
		"Questions":        srv.Questions,
		"AlreadySubmitted": alreadySubmitted,
	})
}

func (h *PublicHandler) SubmitSurvey(w http.ResponseWriter, r *http.Request) {
	slugSecret := r.PathValue("slugSecret")
	grp, srv, _, err := h.resolve(r, slugSecret)
	if err != nil {
		http.Error(w, "Umfrage nicht verfügbar", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Ungültige Daten", http.StatusBadRequest)
		return
	}

	answers := make(map[string]any)
	for _, q := range srv.Questions {
		val := r.FormValue(q.ID)
		if q.Required && val == "" {
			http.Error(w, fmt.Sprintf("Frage %q ist erforderlich", q.Text), http.StatusBadRequest)
			return
		}
		switch q.Type {
		case survey.TypeStars:
			if val != "" {
				n, err := strconv.Atoi(val)
				if err != nil || n < 1 || n > 5 {
					http.Error(w, "Ungültige Bewertung", http.StatusBadRequest)
					return
				}
				answers[q.ID] = n
			}
		case survey.TypeMultiChoice:
			answers[q.ID] = r.Form[q.ID]
		default:
			answers[q.ID] = val
		}
	}

	if _, err := h.surveys.SubmitResponse(r.Context(), srv.ID, answers); err != nil {
		http.Error(w, "Fehler beim Speichern", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     fmt.Sprintf("feedback-%d", srv.ID),
		Value:    "submitted",
		MaxAge:   86400,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, fmt.Sprintf("/f/%s-%s/thanks", grp.Slug, grp.Secret), http.StatusSeeOther)
}

func (h *PublicHandler) ShowThanks(w http.ResponseWriter, r *http.Request) {
	h.render.Render(w, "public/thanks.html", http.StatusOK, nil)
}

func (h *PublicHandler) showGroupSelect(w http.ResponseWriter, r *http.Request, secret string) {
	ctx := r.Context()
	allGroups, err := h.groups.List(ctx)
	if err != nil {
		http.Error(w, "Fehler", http.StatusInternalServerError)
		return
	}

	var activeGroups []group.Group
	for _, g := range allGroups {
		if _, err := h.surveys.GetActiveForGroup(ctx, g.ID); err == nil {
			activeGroups = append(activeGroups, g)
		}
	}

	h.render.Render(w, "public/select_group.html", http.StatusOK, map[string]any{
		"Groups": activeGroups,
	})
}

func (h *PublicHandler) resolve(r *http.Request, slugSecret string) (*group.Group, *survey.Survey, *evening.Evening, error) {
	if len(slugSecret) < 7 {
		return nil, nil, nil, fmt.Errorf("invalid slug-secret")
	}
	secret := slugSecret[len(slugSecret)-5:]
	slug := slugSecret[:len(slugSecret)-6]

	grp, err := h.groups.GetBySlugAndSecret(r.Context(), slug, secret)
	if err != nil {
		return nil, nil, nil, err
	}

	srv, err := h.surveys.GetActiveForGroup(r.Context(), grp.ID)
	if err != nil {
		return nil, nil, nil, err
	}

	eve, err := h.evenings.GetByID(r.Context(), srv.EveningID)
	if err != nil {
		return nil, nil, nil, err
	}

	return grp, srv, eve, nil
}
