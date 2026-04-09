package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/auth"
	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
	"github.com/rubeen/da-feedback/internal/ui"
)

func main() {
	dev := flag.Bool("dev", false, "enable development mode")
	migrateOnly := flag.Bool("migrate", false, "run migrations and exit")
	flag.Parse()

	dbPath := envOr("DAF_DB_PATH", "feedback.db")

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	if *migrateOnly {
		log.Println("migrations complete")
		return
	}

	// Stores
	groups := group.NewStore(db)
	evenings := evening.NewStore(db)
	surveys := survey.NewStore(db)
	analysisStore := analysis.NewStore(db, surveys)
	sessions := auth.NewSessionStore(db, 7*24*time.Hour)

	// OIDC (optional in dev mode)
	var oidc *auth.OIDCClient
	if issuer := os.Getenv("DAF_OIDC_ISSUER"); issuer != "" {
		baseURL := envOr("DAF_BASE_URL", "http://localhost:8080")
		oidc, err = auth.NewOIDCClient(auth.OIDCConfig{
			Issuer:       issuer,
			ClientID:     os.Getenv("DAF_OIDC_CLIENT_ID"),
			ClientSecret: os.Getenv("DAF_OIDC_CLIENT_SECRET"),
			RedirectURL:  baseURL + "/auth/callback",
			AdminGroup:   os.Getenv("DAF_ADMIN_GROUP"),
			GLGroup:      os.Getenv("DAF_GL_GROUP"),
		})
		if err != nil {
			log.Fatalf("oidc: %v", err)
		}
	}

	// Templates
	tFS := os.DirFS("templates")
	renderer, err := ui.NewRenderer(tFS)
	if err != nil {
		log.Fatalf("renderer: %v", err)
	}

	// Router
	baseURL := envOr("DAF_BASE_URL", "http://localhost:8080")
	mux := ui.NewRouter(ui.RouterConfig{
		BaseURL:  baseURL,
		DevMode:  *dev,
		Groups:   groups,
		Evenings: evenings,
		Surveys:  surveys,
		Analysis: analysisStore,
		Sessions: sessions,
		OIDC:     oidc,
		Renderer: renderer,
	})

	// Auto-Close Goroutine
	autoCloser := survey.NewAutoCloser(surveys, 5*time.Minute)
	autoCloser.Start()
	defer autoCloser.Stop()

	addr := envOr("DAF_ADDR", ":8080")
	if *dev {
		log.Printf("dev mode enabled")
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
