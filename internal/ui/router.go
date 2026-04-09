package ui

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/auth"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

type RouterConfig struct {
	BaseURL  string
	DevMode  bool
	Groups   *group.Store
	Evenings *evening.Store
	Surveys  *survey.Store
	Analysis *analysis.Store
	Sessions *auth.SessionStore
	OIDC     *auth.OIDCClient
	Renderer *Renderer
}

type rateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
}

type visitor struct {
	tokens   float64
	lastSeen time.Time
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{visitors: make(map[string]*visitor)}
	go func() {
		for {
			time.Sleep(3 * time.Minute)
			rl.mu.Lock()
			for ip, v := range rl.visitors {
				if time.Since(v.lastSeen) > 5*time.Minute {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		rl.visitors[ip] = &visitor{tokens: 29, lastSeen: time.Now()}
		return true
	}

	elapsed := time.Since(v.lastSeen).Seconds()
	v.tokens += elapsed * 0.5
	if v.tokens > 30 {
		v.tokens = 30
	}
	v.lastSeen = time.Now()

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

func rateLimitMiddleware(rl *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = fwd
		}
		if !rl.allow(ip) {
			http.Error(w, "Zu viele Anfragen", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func NewRouter(cfg RouterConfig) *http.ServeMux {
	mux := http.NewServeMux()

	// Static files
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Root redirect
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusFound)
	})

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Auth routes
	if cfg.OIDC != nil {
		registerAuthRoutes(mux, cfg.OIDC, cfg.Sessions)
	}

	// Dev login bypass
	if cfg.DevMode {
		registerDevLogin(mux, cfg.Sessions)
	}

	// Auth middleware
	authMw := func(next http.Handler) http.Handler {
		return auth.RequireAuth(cfg.Sessions, next)
	}
	adminMw := func(next http.Handler) http.Handler {
		return auth.RequireRole("admin", next)
	}

	// Public routes
	rl := newRateLimiter()
	pub := NewPublicHandler(cfg.Groups, cfg.Evenings, cfg.Surveys, cfg.Renderer, cfg.BaseURL)
	mux.Handle("GET /f/{slugSecret}", rateLimitMiddleware(rl, http.HandlerFunc(pub.ShowSurvey)))
	mux.Handle("POST /f/{slugSecret}/submit", rateLimitMiddleware(rl, http.HandlerFunc(pub.SubmitSurvey)))
	mux.Handle("GET /f/{slugSecret}/thanks", http.HandlerFunc(pub.ShowThanks))

	// Admin routes
	admin := NewAdminHandler(cfg.Groups, cfg.Evenings, cfg.Surveys, cfg.Sessions, cfg.Renderer, cfg.BaseURL)
	admin.RegisterRoutes(mux, authMw)
	admin.RegisterSurveyRoutes(mux, authMw)
	admin.RegisterUserRoutes(mux, authMw, adminMw)

	// Analysis routes
	anal := NewAnalysisHandler(cfg.Analysis, cfg.Groups, cfg.Evenings, cfg.Surveys, cfg.Renderer)
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

func registerDevLogin(mux *http.ServeMux, sessions *auth.SessionStore) {
	mux.HandleFunc("GET /auth/login", func(w http.ResponseWriter, r *http.Request) {
		user := &auth.User{
			ID:    "dev-admin",
			Name:  "Dev Admin",
			Email: "admin@localhost",
			Role:  "admin",
		}
		sess, err := sessions.Create(r.Context(), user)
		if err != nil {
			http.Error(w, "Session error: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "daf_session",
			Value:    sess.ID,
			MaxAge:   int(7 * 24 * time.Hour / time.Second),
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		})
		log.Println("dev login: created admin session")
		http.Redirect(w, r, "/admin", http.StatusFound)
	})
}
