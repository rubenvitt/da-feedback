package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/rubeen/da-feedback/internal/database"
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

	addr := envOr("DAF_ADDR", ":8080")
	if *dev {
		log.Printf("dev mode enabled")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

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
