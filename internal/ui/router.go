package ui

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/auth"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

type RouterConfig struct {
	BaseURL  string
	Groups   *group.Store
	Evenings *evening.Store
	Surveys  *survey.Store
	Analysis *analysis.Store
	Sessions *auth.SessionStore
	OIDC     *auth.OIDCClient
	Renderer *Renderer
}

func NewRouter(cfg RouterConfig) *http.ServeMux {
	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Auth routes
	if cfg.OIDC != nil {
		registerAuthRoutes(mux, cfg.OIDC, cfg.Sessions)
	}

	// Auth middleware
	authMw := func(next http.Handler) http.Handler {
		return auth.RequireAuth(cfg.Sessions, next)
	}
	adminMw := func(next http.Handler) http.Handler {
		return auth.RequireRole("admin", next)
	}

	// Public routes
	pub := NewPublicHandler(cfg.Groups, cfg.Evenings, cfg.Surveys, cfg.Renderer, cfg.BaseURL)
	pub.RegisterRoutes(mux)

	// Admin routes
	admin := NewAdminHandler(cfg.Groups, cfg.Evenings, cfg.Surveys, cfg.Sessions, cfg.Renderer, cfg.BaseURL)
	admin.RegisterRoutes(mux, authMw)
	admin.RegisterSurveyRoutes(mux, authMw)
	admin.RegisterUserRoutes(mux, authMw, adminMw)

	// Analysis routes
	anal := NewAnalysisHandler(cfg.Analysis, cfg.Groups, cfg.Renderer)
	anal.RegisterRoutes(mux, authMw, adminMw)

	return mux
}

func registerAuthRoutes(mux *http.ServeMux, oidc *auth.OIDCClient, sessions *auth.SessionStore) {
	mux.HandleFunc("GET /auth/login", func(w http.ResponseWriter, r *http.Request) {
		state := generateState()
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_state",
			Value:    state,
			MaxAge:   300,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, oidc.AuthURL(state), http.StatusFound)
	})

	mux.HandleFunc("GET /auth/callback", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("oauth_state")
		if err != nil || cookie.Value != r.URL.Query().Get("state") {
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}

		code := r.URL.Query().Get("code")
		user, err := oidc.Exchange(r.Context(), code)
		if err != nil {
			http.Error(w, "Authentication failed: "+err.Error(), http.StatusForbidden)
			return
		}

		sess, err := sessions.Create(r.Context(), user)
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "daf_session",
			Value:    sess.ID,
			MaxAge:   int(7 * 24 * time.Hour / time.Second),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})

		http.Redirect(w, r, "/admin", http.StatusFound)
	})

	mux.HandleFunc("GET /auth/logout", func(w http.ResponseWriter, r *http.Request) {
		if cookie, err := r.Cookie("daf_session"); err == nil {
			sessions.Delete(r.Context(), cookie.Value)
		}
		http.SetCookie(w, &http.Cookie{
			Name:   "daf_session",
			MaxAge: -1,
			Path:   "/",
		})
		http.Redirect(w, r, "/", http.StatusFound)
	})
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
