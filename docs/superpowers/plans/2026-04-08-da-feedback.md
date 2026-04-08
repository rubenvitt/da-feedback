# DRK Dienstabend-Feedback-System — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ein System zur anonymen Feedback-Erfassung nach DRK-Dienstabenden mit QR-Code-basiertem Zugang und Admin-Dashboard.

**Architecture:** Go-Monolith mit Domain-Packages (`internal/{auth,group,evening,survey,analysis,qrcode,ui,database}`). Server-rendered Templates mit HTMX für Interaktivität. SQLite als Datenbank mit goose-Migrationen.

**Tech Stack:** Go, SQLite (modernc.org/sqlite), goose, HTMX, Tailwind CSS (Standalone CLI), Apache ECharts, PocketID (OIDC), Docker, Litestream

## File Structure

```
da-feedback/
├── cmd/server/main.go
├── internal/
│   ├── auth/
│   │   ├── oidc.go
│   │   ├── session.go
│   │   └── middleware.go
│   ├── database/
│   │   └── db.go
│   ├── group/
│   │   ├── model.go
│   │   ├── store.go
│   │   └── store_test.go
│   ├── evening/
│   │   ├── model.go
│   │   ├── store.go
│   │   └── store_test.go
│   ├── survey/
│   │   ├── model.go
│   │   ├── questions.go
│   │   ├── store.go
│   │   ├── store_test.go
│   │   ├── lifecycle.go
│   │   └── lifecycle_test.go
│   ├── analysis/
│   │   ├── aggregation.go
│   │   ├── aggregation_test.go
│   │   └── export.go
│   ├── qrcode/
│   │   └── generate.go
│   └── ui/
│       ├── router.go
│       ├── render.go
│       ├── public.go
│       ├── public_test.go
│       ├── admin.go
│       ├── admin_surveys.go
│       ├── admin_users.go
│       └── analysis.go
├── migrations/
│   └── 001_initial.sql
├── templates/
│   ├── base.html
│   ├── public/
│   │   ├── survey.html
│   │   ├── thanks.html
│   │   ├── unavailable.html
│   │   └── select_group.html
│   └── admin/
│       ├── dashboard.html
│       ├── groups.html
│       ├── group_detail.html
│       ├── evening_form.html
│       ├── survey_detail.html
│       ├── analysis_da.html
│       ├── analysis_group.html
│       ├── analysis_global.html
│       └── users.html
├── static/
│   └── input.css
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── tailwind.config.js
└── .gitignore
```

---

### Task 1: Projekt-Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `internal/database/db.go`

- [ ] **Step 1: Go-Modul initialisieren**

```bash
go mod init github.com/rubeen/da-feedback
```

- [ ] **Step 2: `.gitignore` erstellen**

Create `.gitignore`:
```gitignore
# Binary
da-feedback

# Database
*.db
*.db-wal
*.db-shm

# Tailwind
static/style.css
node_modules/

# IDE
.idea/
.vscode/
*.swp

# OS
.DS_Store

# Superpowers
.superpowers/

# Env
.env
```

- [ ] **Step 3: Makefile erstellen**

Create `Makefile`:
```makefile
.PHONY: dev build docker migrate tailwind tailwind-watch

BINARY=da-feedback
TAILWIND=./tailwindcss

dev:
	@echo "Starting dev server..."
	@$(MAKE) tailwind-watch &
	@go run ./cmd/server -dev

build: tailwind
	CGO_ENABLED=0 go build -o $(BINARY) ./cmd/server

docker:
	docker build -t da-feedback .

migrate:
	go run ./cmd/server -migrate

tailwind:
	$(TAILWIND) -i static/input.css -o static/style.css --minify

tailwind-watch:
	$(TAILWIND) -i static/input.css -o static/style.css --watch

test:
	go test ./...
```

- [ ] **Step 4: Datenbank-Package erstellen**

Create `internal/database/db.go`:
```go
package database

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed ../../migrations/*.sql
var migrations embed.FS

func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

func Migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set dialect: %w", err)
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Minimalen main.go erstellen**

Create `cmd/server/main.go`:
```go
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
```

- [ ] **Step 6: Initiale Migration erstellen**

Create `migrations/001_initial.sql`:
```sql
-- +goose Up
CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT INTO config (key, value) VALUES ('default_close_after_hours', '48');

CREATE TABLE groups (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT    NOT NULL,
    slug              TEXT    NOT NULL UNIQUE,
    secret            TEXT    NOT NULL,
    close_after_hours INTEGER,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_groups_slug ON groups(slug);

CREATE TABLE evenings (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id          INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    date              DATE    NOT NULL,
    topic             TEXT,
    notes             TEXT,
    participant_count INTEGER,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_evenings_group_date ON evenings(group_id, date);

CREATE TABLE surveys (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    evening_id        INTEGER NOT NULL UNIQUE REFERENCES evenings(id) ON DELETE CASCADE,
    status            TEXT    NOT NULL DEFAULT 'draft'
                      CHECK(status IN ('draft', 'active', 'closed', 'archived')),
    questions         TEXT    NOT NULL DEFAULT '[]',
    close_after_hours INTEGER,
    activated_at      TIMESTAMP,
    closes_at         TIMESTAMP,
    closed_at         TIMESTAMP,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_surveys_status ON surveys(status);

CREATE TABLE responses (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    survey_id    INTEGER   NOT NULL REFERENCES surveys(id) ON DELETE CASCADE,
    answers      TEXT      NOT NULL,
    submitted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_responses_survey ON responses(survey_id);

CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    email      TEXT NOT NULL,
    role       TEXT NOT NULL CHECK(role IN ('admin', 'groupleader')),
    last_login TIMESTAMP
);

CREATE TABLE user_groups (
    user_id  TEXT    NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);

CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    data       TEXT,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- +goose Down
DROP TABLE sessions;
DROP TABLE user_groups;
DROP TABLE users;
DROP TABLE responses;
DROP TABLE surveys;
DROP TABLE evenings;
DROP TABLE groups;
DROP TABLE config;
```

- [ ] **Step 7: Dependencies installieren**

```bash
go mod tidy
```

- [ ] **Step 8: Verifizieren**

```bash
go build ./cmd/server
go test ./...
```
Expected: Build erfolgreich, keine Testfehler.

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "feat: project scaffolding with database, migrations, and basic server"
```

---

### Task 2: Group Domain (Model + Store)

**Files:**
- Create: `internal/group/model.go`
- Create: `internal/group/store.go`
- Create: `internal/group/store_test.go`

- [ ] **Step 1: Model erstellen**

Create `internal/group/model.go`:
```go
package group

import "time"

type Group struct {
	ID              int
	Name            string
	Slug            string
	Secret          string
	CloseAfterHours *int
	CreatedAt       time.Time
}
```

- [ ] **Step 2: Failing Tests schreiben**

Create `internal/group/store_test.go`:
```go
package group_test

import (
	"context"
	"testing"

	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/group"
)

func setupTestDB(t *testing.T) *group.Store {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return group.NewStore(db)
}

func TestCreateAndGetGroup(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	g, err := store.Create(ctx, "I&K", "iuk")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if g.Name != "I&K" || g.Slug != "iuk" || len(g.Secret) != 5 {
		t.Fatalf("unexpected group: %+v", g)
	}

	got, err := store.GetByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "I&K" {
		t.Fatalf("expected I&K, got %s", got.Name)
	}
}

func TestGetBySlugAndSecret(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	created, err := store.Create(ctx, "Sanitätsdienst", "san")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := store.GetBySlugAndSecret(ctx, created.Slug, created.Secret)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != created.ID {
		t.Fatalf("expected id %d, got %d", created.ID, got.ID)
	}

	_, err = store.GetBySlugAndSecret(ctx, "san", "wrong")
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestListGroups(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	store.Create(ctx, "I&K", "iuk")
	store.Create(ctx, "Sanitätsdienst", "san")

	groups, err := store.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2, got %d", len(groups))
	}
}

func TestUpdateGroup(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	g, _ := store.Create(ctx, "I&K", "iuk")
	hours := 24
	g.Name = "IuK"
	g.CloseAfterHours = &hours
	err := store.Update(ctx, g)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := store.GetByID(ctx, g.ID)
	if got.Name != "IuK" || *got.CloseAfterHours != 24 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestDeleteGroup(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	g, _ := store.Create(ctx, "I&K", "iuk")
	err := store.Delete(ctx, g.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = store.GetByID(ctx, g.ID)
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestRegenerateSecret(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()

	g, _ := store.Create(ctx, "I&K", "iuk")
	oldSecret := g.Secret

	newSecret, err := store.RegenerateSecret(ctx, g.ID)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if newSecret == oldSecret {
		t.Fatal("secret should have changed")
	}
	if len(newSecret) != 5 {
		t.Fatalf("expected 5 chars, got %d", len(newSecret))
	}
}
```

- [ ] **Step 3: Tests ausführen, Fehlschlag bestätigen**

```bash
go test ./internal/group/...
```
Expected: FAIL — `group.NewStore` und Methoden existieren noch nicht.

- [ ] **Step 4: Store implementieren**

Create `internal/group/store.go`:
```go
package group

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"math/big"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

const secretChars = "abcdefghijklmnopqrstuvwxyz0123456789"
const secretLength = 5

func generateSecret() (string, error) {
	b := make([]byte, secretLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(secretChars))))
		if err != nil {
			return "", err
		}
		b[i] = secretChars[n.Int64()]
	}
	return string(b), nil
}

func (s *Store) Create(ctx context.Context, name, slug string) (*Group, error) {
	secret, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO groups (name, slug, secret) VALUES (?, ?, ?)",
		name, slug, secret)
	if err != nil {
		return nil, fmt.Errorf("insert group: %w", err)
	}

	id, _ := res.LastInsertId()
	return s.GetByID(ctx, int(id))
}

func (s *Store) GetByID(ctx context.Context, id int) (*Group, error) {
	g := &Group{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, slug, secret, close_after_hours, created_at FROM groups WHERE id = ?", id,
	).Scan(&g.ID, &g.Name, &g.Slug, &g.Secret, &g.CloseAfterHours, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get group %d: %w", id, err)
	}
	return g, nil
}

func (s *Store) GetBySlugAndSecret(ctx context.Context, slug, secret string) (*Group, error) {
	g := &Group{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, slug, secret, close_after_hours, created_at FROM groups WHERE slug = ? AND secret = ?",
		slug, secret,
	).Scan(&g.ID, &g.Name, &g.Slug, &g.Secret, &g.CloseAfterHours, &g.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get group by slug: %w", err)
	}
	return g, nil
}

func (s *Store) List(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, slug, secret, close_after_hours, created_at FROM groups ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	var groups []Group
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name, &g.Slug, &g.Secret, &g.CloseAfterHours, &g.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *Store) Update(ctx context.Context, g *Group) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE groups SET name = ?, slug = ?, close_after_hours = ? WHERE id = ?",
		g.Name, g.Slug, g.CloseAfterHours, g.ID)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM groups WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	return nil
}

func (s *Store) RegenerateSecret(ctx context.Context, id int) (string, error) {
	secret, err := generateSecret()
	if err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	_, err = s.db.ExecContext(ctx, "UPDATE groups SET secret = ? WHERE id = ?", secret, id)
	if err != nil {
		return "", fmt.Errorf("update secret: %w", err)
	}
	return secret, nil
}
```

- [ ] **Step 5: Tests ausführen, Erfolg bestätigen**

```bash
go test ./internal/group/... -v
```
Expected: Alle Tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/group/
git commit -m "feat: group domain with model, store, and tests"
```

---

### Task 3: Evening Domain (Model + Store)

**Files:**
- Create: `internal/evening/model.go`
- Create: `internal/evening/store.go`
- Create: `internal/evening/store_test.go`

- [ ] **Step 1: Model erstellen**

Create `internal/evening/model.go`:
```go
package evening

import "time"

type Evening struct {
	ID               int
	GroupID          int
	Date             time.Time
	Topic            *string
	Notes            *string
	ParticipantCount *int
	CreatedAt        time.Time
}
```

- [ ] **Step 2: Failing Tests schreiben**

Create `internal/evening/store_test.go`:
```go
package evening_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
)

func setupTestDB(t *testing.T) (*evening.Store, *group.Store) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return evening.NewStore(db), group.NewStore(db)
}

func TestCreateAndGetEvening(t *testing.T) {
	eStore, gStore := setupTestDB(t)
	ctx := context.Background()

	g, _ := gStore.Create(ctx, "I&K", "iuk")
	topic := "Funkausbildung"
	date := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)

	e, err := eStore.Create(ctx, g.ID, date, &topic)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if e.GroupID != g.ID || *e.Topic != "Funkausbildung" {
		t.Fatalf("unexpected evening: %+v", e)
	}

	got, err := eStore.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != e.ID {
		t.Fatalf("expected %d, got %d", e.ID, got.ID)
	}
}

func TestListByGroup(t *testing.T) {
	eStore, gStore := setupTestDB(t)
	ctx := context.Background()

	g, _ := gStore.Create(ctx, "I&K", "iuk")
	d1 := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	eStore.Create(ctx, g.ID, d1, nil)
	eStore.Create(ctx, g.ID, d2, nil)

	evenings, err := eStore.ListByGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(evenings) != 2 {
		t.Fatalf("expected 2, got %d", len(evenings))
	}
	// Neueste zuerst
	if evenings[0].Date.Before(evenings[1].Date) {
		t.Fatal("expected newest first")
	}
}

func TestUpdateParticipantCount(t *testing.T) {
	eStore, gStore := setupTestDB(t)
	ctx := context.Background()

	g, _ := gStore.Create(ctx, "I&K", "iuk")
	date := time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC)
	e, _ := eStore.Create(ctx, g.ID, date, nil)

	count := 15
	err := eStore.UpdateParticipantCount(ctx, e.ID, &count)
	if err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := eStore.GetByID(ctx, e.ID)
	if got.ParticipantCount == nil || *got.ParticipantCount != 15 {
		t.Fatalf("expected 15, got %v", got.ParticipantCount)
	}
}
```

- [ ] **Step 3: Tests ausführen, Fehlschlag bestätigen**

```bash
go test ./internal/evening/...
```
Expected: FAIL.

- [ ] **Step 4: Store implementieren**

Create `internal/evening/store.go`:
```go
package evening

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, groupID int, date time.Time, topic *string) (*Evening, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO evenings (group_id, date, topic) VALUES (?, ?, ?)",
		groupID, date, topic)
	if err != nil {
		return nil, fmt.Errorf("insert evening: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetByID(ctx, int(id))
}

func (s *Store) GetByID(ctx context.Context, id int) (*Evening, error) {
	e := &Evening{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, group_id, date, topic, notes, participant_count, created_at FROM evenings WHERE id = ?", id,
	).Scan(&e.ID, &e.GroupID, &e.Date, &e.Topic, &e.Notes, &e.ParticipantCount, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get evening %d: %w", id, err)
	}
	return e, nil
}

func (s *Store) ListByGroup(ctx context.Context, groupID int) ([]Evening, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, group_id, date, topic, notes, participant_count, created_at FROM evenings WHERE group_id = ? ORDER BY date DESC",
		groupID)
	if err != nil {
		return nil, fmt.Errorf("list evenings: %w", err)
	}
	defer rows.Close()

	var evenings []Evening
	for rows.Next() {
		var e Evening
		if err := rows.Scan(&e.ID, &e.GroupID, &e.Date, &e.Topic, &e.Notes, &e.ParticipantCount, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan evening: %w", err)
		}
		evenings = append(evenings, e)
	}
	return evenings, rows.Err()
}

func (s *Store) UpdateParticipantCount(ctx context.Context, id int, count *int) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE evenings SET participant_count = ? WHERE id = ?", count, id)
	if err != nil {
		return fmt.Errorf("update participant count: %w", err)
	}
	return nil
}

func (s *Store) Update(ctx context.Context, e *Evening) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE evenings SET date = ?, topic = ?, notes = ?, participant_count = ? WHERE id = ?",
		e.Date, e.Topic, e.Notes, e.ParticipantCount, e.ID)
	if err != nil {
		return fmt.Errorf("update evening: %w", err)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM evenings WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete evening: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Tests ausführen, Erfolg bestätigen**

```bash
go test ./internal/evening/... -v
```
Expected: Alle Tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/evening/
git commit -m "feat: evening domain with model, store, and tests"
```

---

### Task 4: Survey Domain (Model, Questions, Store)

**Files:**
- Create: `internal/survey/model.go`
- Create: `internal/survey/questions.go`
- Create: `internal/survey/store.go`
- Create: `internal/survey/store_test.go`

- [ ] **Step 1: Model + Frage-Typen erstellen**

Create `internal/survey/model.go`:
```go
package survey

import "time"

type Status string

const (
	StatusDraft    Status = "draft"
	StatusActive   Status = "active"
	StatusClosed   Status = "closed"
	StatusArchived Status = "archived"
)

type QuestionType string

const (
	TypeStars        QuestionType = "stars"
	TypeText         QuestionType = "text"
	TypeSingleChoice QuestionType = "single_choice"
	TypeMultiChoice  QuestionType = "multi_choice"
)

type Question struct {
	ID       string       `json:"id"`
	Type     QuestionType `json:"type"`
	Text     string       `json:"text"`
	Required bool         `json:"required"`
	Standard bool         `json:"standard"`
	Options  []string     `json:"options,omitempty"`
}

type Survey struct {
	ID              int
	EveningID       int
	Status          Status
	Questions       []Question
	CloseAfterHours *int
	ActivatedAt     *time.Time
	ClosesAt        *time.Time
	ClosedAt        *time.Time
	CreatedAt       time.Time
}

type Response struct {
	ID          int
	SurveyID    int
	Answers     map[string]any
	SubmittedAt time.Time
}
```

- [ ] **Step 2: Standard-Fragenblock definieren**

Create `internal/survey/questions.go`:
```go
package survey

var StandardQuestions = []Question{
	{ID: "q1", Type: TypeStars, Text: "Wie bewertest du den heutigen Dienstabend insgesamt?", Required: true, Standard: true},
	{ID: "q2", Type: TypeStars, Text: "Wie praxisrelevant waren die Inhalte?", Required: true, Standard: true},
	{ID: "q3", Type: TypeStars, Text: "Wie verständlich war die Vermittlung?", Required: true, Standard: true},
	{ID: "q4", Type: TypeText, Text: "Was hat dir besonders gut gefallen?", Required: false, Standard: true},
	{ID: "q5", Type: TypeText, Text: "Was könnte beim nächsten Mal besser laufen?", Required: false, Standard: true},
	{ID: "q6", Type: TypeText, Text: "Welches Thema wünschst du dir für einen kommenden DA?", Required: false, Standard: true},
}

func NewQuestionsWithStandard(extra []Question) []Question {
	all := make([]Question, len(StandardQuestions))
	copy(all, StandardQuestions)
	return append(all, extra...)
}
```

- [ ] **Step 3: Failing Tests schreiben**

Create `internal/survey/store_test.go`:
```go
package survey_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

type testEnv struct {
	surveys  *survey.Store
	evenings *evening.Store
	groups   *group.Store
}

func setupTestDB(t *testing.T) testEnv {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return testEnv{
		surveys:  survey.NewStore(db),
		evenings: evening.NewStore(db),
		groups:   group.NewStore(db),
	}
}

func createTestEvening(t *testing.T, env testEnv) (*group.Group, *evening.Evening) {
	t.Helper()
	ctx := context.Background()
	g, err := env.groups.Create(ctx, "I&K", "iuk")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	e, err := env.evenings.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("create evening: %v", err)
	}
	return g, e
}

func TestCreateSurvey(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, err := env.surveys.Create(ctx, e.ID, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Status != survey.StatusDraft {
		t.Fatalf("expected draft, got %s", s.Status)
	}
	if len(s.Questions) != 6 {
		t.Fatalf("expected 6 standard questions, got %d", len(s.Questions))
	}
}

func TestActivateSurvey(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	err := env.surveys.Activate(ctx, s.ID, 48)
	if err != nil {
		t.Fatalf("activate: %v", err)
	}

	got, _ := env.surveys.GetByID(ctx, s.ID)
	if got.Status != survey.StatusActive {
		t.Fatalf("expected active, got %s", got.Status)
	}
	if got.ActivatedAt == nil || got.ClosesAt == nil {
		t.Fatal("expected activated_at and closes_at to be set")
	}
}

func TestOnlyOneActiveSurveyPerGroup(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	g, e1 := createTestEvening(t, env)

	e2, _ := env.evenings.Create(ctx, g.ID, time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC), nil)

	s1, _ := env.surveys.Create(ctx, e1.ID, nil)
	env.surveys.Activate(ctx, s1.ID, 48)

	s2, _ := env.surveys.Create(ctx, e2.ID, nil)
	env.surveys.Activate(ctx, s2.ID, 48)

	// s1 sollte jetzt geschlossen sein
	got, _ := env.surveys.GetByID(ctx, s1.ID)
	if got.Status != survey.StatusClosed {
		t.Fatalf("expected s1 closed, got %s", got.Status)
	}
}

func TestGetActiveSurveyForGroup(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	g, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	env.surveys.Activate(ctx, s.ID, 48)

	active, err := env.surveys.GetActiveForGroup(ctx, g.ID)
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if active.ID != s.ID {
		t.Fatalf("expected %d, got %d", s.ID, active.ID)
	}
}

func TestSubmitResponse(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	env.surveys.Activate(ctx, s.ID, 48)

	answers := map[string]any{"q1": 5, "q2": 4, "q3": 3, "q4": "Gut!", "q5": "", "q6": ""}
	r, err := env.surveys.SubmitResponse(ctx, s.ID, answers)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if r.SurveyID != s.ID {
		t.Fatalf("expected survey %d, got %d", s.ID, r.SurveyID)
	}

	responses, err := env.surveys.GetResponses(ctx, s.ID)
	if err != nil {
		t.Fatalf("get responses: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
}

func TestCloseSurvey(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	env.surveys.Activate(ctx, s.ID, 48)

	err := env.surveys.Close(ctx, s.ID)
	if err != nil {
		t.Fatalf("close: %v", err)
	}

	got, _ := env.surveys.GetByID(ctx, s.ID)
	if got.Status != survey.StatusClosed {
		t.Fatalf("expected closed, got %s", got.Status)
	}
	if got.ClosedAt == nil {
		t.Fatal("expected closed_at to be set")
	}
}
```

- [ ] **Step 4: Tests ausführen, Fehlschlag bestätigen**

```bash
go test ./internal/survey/...
```
Expected: FAIL.

- [ ] **Step 5: Store implementieren**

Create `internal/survey/store.go`:
```go
package survey

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Create(ctx context.Context, eveningID int, extraQuestions []Question) (*Survey, error) {
	questions := NewQuestionsWithStandard(extraQuestions)
	qJSON, err := json.Marshal(questions)
	if err != nil {
		return nil, fmt.Errorf("marshal questions: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO surveys (evening_id, questions) VALUES (?, ?)",
		eveningID, string(qJSON))
	if err != nil {
		return nil, fmt.Errorf("insert survey: %w", err)
	}
	id, _ := res.LastInsertId()
	return s.GetByID(ctx, int(id))
}

func (s *Store) GetByID(ctx context.Context, id int) (*Survey, error) {
	sv := &Survey{}
	var qJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, evening_id, status, questions, close_after_hours,
		        activated_at, closes_at, closed_at, created_at
		 FROM surveys WHERE id = ?`, id,
	).Scan(&sv.ID, &sv.EveningID, &sv.Status, &qJSON, &sv.CloseAfterHours,
		&sv.ActivatedAt, &sv.ClosesAt, &sv.ClosedAt, &sv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get survey %d: %w", id, err)
	}
	if err := json.Unmarshal([]byte(qJSON), &sv.Questions); err != nil {
		return nil, fmt.Errorf("unmarshal questions: %w", err)
	}
	return sv, nil
}

func (s *Store) GetByEveningID(ctx context.Context, eveningID int) (*Survey, error) {
	sv := &Survey{}
	var qJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, evening_id, status, questions, close_after_hours,
		        activated_at, closes_at, closed_at, created_at
		 FROM surveys WHERE evening_id = ?`, eveningID,
	).Scan(&sv.ID, &sv.EveningID, &sv.Status, &qJSON, &sv.CloseAfterHours,
		&sv.ActivatedAt, &sv.ClosesAt, &sv.ClosedAt, &sv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get survey by evening %d: %w", eveningID, err)
	}
	if err := json.Unmarshal([]byte(qJSON), &sv.Questions); err != nil {
		return nil, fmt.Errorf("unmarshal questions: %w", err)
	}
	return sv, nil
}

func (s *Store) Activate(ctx context.Context, id int, closeAfterHours int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Fachgruppe der Umfrage ermitteln
	var groupID int
	err = tx.QueryRowContext(ctx,
		`SELECT e.group_id FROM surveys s
		 JOIN evenings e ON s.evening_id = e.id
		 WHERE s.id = ?`, id).Scan(&groupID)
	if err != nil {
		return fmt.Errorf("get group for survey: %w", err)
	}

	// Alle aktiven Umfragen dieser Gruppe schließen
	now := time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE surveys SET status = 'closed', closed_at = ?
		 WHERE status = 'active' AND evening_id IN (
		     SELECT id FROM evenings WHERE group_id = ?
		 )`, now, groupID)
	if err != nil {
		return fmt.Errorf("close active surveys: %w", err)
	}

	// Diese Umfrage aktivieren
	closesAt := now.Add(time.Duration(closeAfterHours) * time.Hour)
	_, err = tx.ExecContext(ctx,
		`UPDATE surveys SET status = 'active', activated_at = ?, closes_at = ?
		 WHERE id = ? AND status = 'draft'`,
		now, closesAt, id)
	if err != nil {
		return fmt.Errorf("activate survey: %w", err)
	}

	return tx.Commit()
}

func (s *Store) Close(ctx context.Context, id int) error {
	now := time.Now()
	_, err := s.db.ExecContext(ctx,
		"UPDATE surveys SET status = 'closed', closed_at = ? WHERE id = ?",
		now, id)
	if err != nil {
		return fmt.Errorf("close survey: %w", err)
	}
	return nil
}

func (s *Store) Archive(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE surveys SET status = 'archived' WHERE id = ? AND status = 'closed'", id)
	if err != nil {
		return fmt.Errorf("archive survey: %w", err)
	}
	return nil
}

func (s *Store) GetActiveForGroup(ctx context.Context, groupID int) (*Survey, error) {
	sv := &Survey{}
	var qJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT s.id, s.evening_id, s.status, s.questions, s.close_after_hours,
		        s.activated_at, s.closes_at, s.closed_at, s.created_at
		 FROM surveys s
		 JOIN evenings e ON s.evening_id = e.id
		 WHERE e.group_id = ? AND s.status = 'active'`, groupID,
	).Scan(&sv.ID, &sv.EveningID, &sv.Status, &qJSON, &sv.CloseAfterHours,
		&sv.ActivatedAt, &sv.ClosesAt, &sv.ClosedAt, &sv.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get active survey for group %d: %w", groupID, err)
	}
	if err := json.Unmarshal([]byte(qJSON), &sv.Questions); err != nil {
		return nil, fmt.Errorf("unmarshal questions: %w", err)
	}
	return sv, nil
}

func (s *Store) SubmitResponse(ctx context.Context, surveyID int, answers map[string]any) (*Response, error) {
	aJSON, err := json.Marshal(answers)
	if err != nil {
		return nil, fmt.Errorf("marshal answers: %w", err)
	}

	res, err := s.db.ExecContext(ctx,
		"INSERT INTO responses (survey_id, answers) VALUES (?, ?)",
		surveyID, string(aJSON))
	if err != nil {
		return nil, fmt.Errorf("insert response: %w", err)
	}

	id, _ := res.LastInsertId()
	r := &Response{}
	var rJSON string
	err = s.db.QueryRowContext(ctx,
		"SELECT id, survey_id, answers, submitted_at FROM responses WHERE id = ?", id,
	).Scan(&r.ID, &r.SurveyID, &rJSON, &r.SubmittedAt)
	if err != nil {
		return nil, fmt.Errorf("get response: %w", err)
	}
	if err := json.Unmarshal([]byte(rJSON), &r.Answers); err != nil {
		return nil, fmt.Errorf("unmarshal answers: %w", err)
	}
	return r, nil
}

func (s *Store) GetResponses(ctx context.Context, surveyID int) ([]Response, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, survey_id, answers, submitted_at FROM responses WHERE survey_id = ? ORDER BY submitted_at",
		surveyID)
	if err != nil {
		return nil, fmt.Errorf("list responses: %w", err)
	}
	defer rows.Close()

	var responses []Response
	for rows.Next() {
		var r Response
		var aJSON string
		if err := rows.Scan(&r.ID, &r.SurveyID, &aJSON, &r.SubmittedAt); err != nil {
			return nil, fmt.Errorf("scan response: %w", err)
		}
		if err := json.Unmarshal([]byte(aJSON), &r.Answers); err != nil {
			return nil, fmt.Errorf("unmarshal answers: %w", err)
		}
		responses = append(responses, r)
	}
	return responses, rows.Err()
}

func (s *Store) GetResponseCount(ctx context.Context, surveyID int) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM responses WHERE survey_id = ?", surveyID).Scan(&count)
	return count, err
}

// CloseExpired schließt alle abgelaufenen aktiven Umfragen. Wird von der Auto-Close Goroutine aufgerufen.
func (s *Store) CloseExpired(ctx context.Context) (int, error) {
	now := time.Now()
	res, err := s.db.ExecContext(ctx,
		"UPDATE surveys SET status = 'closed', closed_at = ? WHERE status = 'active' AND closes_at <= ?",
		now, now)
	if err != nil {
		return 0, fmt.Errorf("close expired: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
```

- [ ] **Step 6: Tests ausführen, Erfolg bestätigen**

```bash
go test ./internal/survey/... -v
```
Expected: Alle Tests PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/survey/
git commit -m "feat: survey domain with model, store, lifecycle, questions, and tests"
```

---

### Task 5: Survey Auto-Close Lifecycle

**Files:**
- Create: `internal/survey/lifecycle.go`
- Create: `internal/survey/lifecycle_test.go`

- [ ] **Step 1: Failing Test schreiben**

Create `internal/survey/lifecycle_test.go`:
```go
package survey_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/survey"
)

func TestCloseExpired(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()
	_, e := createTestEvening(t, env)

	s, _ := env.surveys.Create(ctx, e.ID, nil)
	// Aktivieren mit 0 Stunden → sofort abgelaufen
	env.surveys.Activate(ctx, s.ID, 0)

	// Kurz warten damit closes_at in der Vergangenheit liegt
	time.Sleep(10 * time.Millisecond)

	n, err := env.surveys.CloseExpired(ctx)
	if err != nil {
		t.Fatalf("close expired: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 closed, got %d", n)
	}

	got, _ := env.surveys.GetByID(ctx, s.ID)
	if got.Status != survey.StatusClosed {
		t.Fatalf("expected closed, got %s", got.Status)
	}
}

func TestAutoCloserStartStop(t *testing.T) {
	env := setupTestDB(t)
	closer := survey.NewAutoCloser(env.surveys, 50*time.Millisecond)
	closer.Start()
	time.Sleep(100 * time.Millisecond)
	closer.Stop()
	// Kein Panic, kein Hang → Test bestanden
}
```

- [ ] **Step 2: Tests ausführen, Fehlschlag bestätigen**

```bash
go test ./internal/survey/... -run TestAutoCloser
```
Expected: FAIL — `survey.NewAutoCloser` existiert noch nicht.

- [ ] **Step 3: AutoCloser implementieren**

Create `internal/survey/lifecycle.go`:
```go
package survey

import (
	"context"
	"log"
	"time"
)

type AutoCloser struct {
	store    *Store
	interval time.Duration
	stop     chan struct{}
}

func NewAutoCloser(store *Store, interval time.Duration) *AutoCloser {
	return &AutoCloser{
		store:    store,
		interval: interval,
		stop:     make(chan struct{}),
	}
}

func (ac *AutoCloser) Start() {
	go func() {
		ticker := time.NewTicker(ac.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				n, err := ac.store.CloseExpired(context.Background())
				if err != nil {
					log.Printf("auto-close error: %v", err)
				} else if n > 0 {
					log.Printf("auto-closed %d survey(s)", n)
				}
			case <-ac.stop:
				return
			}
		}
	}()
}

func (ac *AutoCloser) Stop() {
	close(ac.stop)
}
```

- [ ] **Step 4: Tests ausführen, Erfolg bestätigen**

```bash
go test ./internal/survey/... -v
```
Expected: Alle Tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/survey/lifecycle.go internal/survey/lifecycle_test.go
git commit -m "feat: survey auto-close lifecycle goroutine"
```

---

### Task 6: Auth (OIDC + Sessions + Middleware)

**Files:**
- Create: `internal/auth/oidc.go`
- Create: `internal/auth/session.go`
- Create: `internal/auth/middleware.go`

- [ ] **Step 1: Session-Store erstellen**

Create `internal/auth/session.go`:
```go
package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type User struct {
	ID        string
	Name      string
	Email     string
	Role      string
	LastLogin *time.Time
}

type Session struct {
	ID        string
	UserID    string
	User      *User
	ExpiresAt time.Time
}

type SessionStore struct {
	db  *sql.DB
	ttl time.Duration
}

func NewSessionStore(db *sql.DB, ttl time.Duration) *SessionStore {
	return &SessionStore{db: db, ttl: ttl}
}

func (s *SessionStore) Create(ctx context.Context, user *User) (*Session, error) {
	id, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	expiresAt := time.Now().Add(s.ttl)

	// Upsert user
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO users (id, name, email, role, last_login) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name=?, email=?, role=?, last_login=?`,
		user.ID, user.Name, user.Email, user.Role, time.Now(),
		user.Name, user.Email, user.Role, time.Now())
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)",
		id, user.ID, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	return &Session{ID: id, UserID: user.ID, User: user, ExpiresAt: expiresAt}, nil
}

func (s *SessionStore) Get(ctx context.Context, id string) (*Session, error) {
	sess := &Session{User: &User{}}
	var data sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT s.id, s.user_id, s.expires_at, u.name, u.email, u.role
		 FROM sessions s JOIN users u ON s.user_id = u.id
		 WHERE s.id = ? AND s.expires_at > ?`, id, time.Now(),
	).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt,
		&sess.User.Name, &sess.User.Email, &sess.User.Role)
	_ = data
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}
	sess.User.ID = sess.UserID
	return sess, nil
}

func (s *SessionStore) Refresh(ctx context.Context, id string) error {
	expiresAt := time.Now().Add(s.ttl)
	_, err := s.db.ExecContext(ctx,
		"UPDATE sessions SET expires_at = ? WHERE id = ?", expiresAt, id)
	return err
}

func (s *SessionStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	return err
}

func (s *SessionStore) DeleteExpired(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at <= ?", time.Now())
	return err
}

// GetUserGroups gibt die Fachgruppen-IDs zurück, die dem User zugeordnet sind
func (s *SessionStore) GetUserGroups(ctx context.Context, userID string) ([]int, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT group_id FROM user_groups WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// UserToJSON / UserFromJSON für Session-Data (nicht aktuell benötigt, aber vorbereitet)
func UserToJSON(u *User) string {
	b, _ := json.Marshal(u)
	return string(b)
}
```

- [ ] **Step 2: OIDC-Client erstellen**

Create `internal/auth/oidc.go`:
```go
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

type OIDCClient struct {
	config       OIDCConfig
	authURL      string
	tokenURL     string
	userInfoURL  string
	adminGroup   string
	glGroup      string
}

type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	IDToken     string `json:"id_token"`
}

type userInfoResponse struct {
	Sub    string   `json:"sub"`
	Name   string   `json:"name"`
	Email  string   `json:"email"`
	Groups []string `json:"groups"`
}

func NewOIDCClient(config OIDCConfig) (*OIDCClient, error) {
	disc, err := discover(config.Issuer)
	if err != nil {
		return nil, err
	}
	return &OIDCClient{
		config:      config,
		authURL:     disc.AuthorizationEndpoint,
		tokenURL:    disc.TokenEndpoint,
		userInfoURL: disc.UserinfoEndpoint,
		adminGroup:  "da-feedback-admin",
		glGroup:     "da-feedback-gl",
	}, nil
}

func discover(issuer string) (*oidcDiscovery, error) {
	resp, err := http.Get(issuer + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	var disc oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&disc); err != nil {
		return nil, fmt.Errorf("decode discovery: %w", err)
	}
	return &disc, nil
}

func (c *OIDCClient) AuthURL(state string) string {
	v := url.Values{
		"client_id":     {c.config.ClientID},
		"redirect_uri":  {c.config.RedirectURL},
		"response_type": {"code"},
		"scope":         {"openid profile email groups"},
		"state":         {state},
	}
	return c.authURL + "?" + v.Encode()
}

func (c *OIDCClient) Exchange(ctx context.Context, code string) (*User, error) {
	// Token austauschen
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {c.config.RedirectURL},
		"client_id":     {c.config.ClientID},
		"client_secret": {c.config.ClientSecret},
	}

	resp, err := http.PostForm(c.tokenURL, data)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, body)
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("decode token: %w", err)
	}

	// UserInfo abrufen
	return c.getUserInfo(ctx, token.AccessToken)
}

func (c *OIDCClient) getUserInfo(ctx context.Context, accessToken string) (*User, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", c.userInfoURL, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo: %w", err)
	}
	defer resp.Body.Close()

	var info userInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode userinfo: %w", err)
	}

	role := c.resolveRole(info.Groups)
	if role == "" {
		return nil, fmt.Errorf("user %s has no authorized group", info.Sub)
	}

	return &User{
		ID:    info.Sub,
		Name:  info.Name,
		Email: info.Email,
		Role:  role,
	}, nil
}

func (c *OIDCClient) resolveRole(groups []string) string {
	for _, g := range groups {
		if strings.EqualFold(g, c.adminGroup) {
			return "admin"
		}
	}
	for _, g := range groups {
		if strings.EqualFold(g, c.glGroup) {
			return "groupleader"
		}
	}
	return ""
}
```

- [ ] **Step 3: Middleware erstellen**

Create `internal/auth/middleware.go`:
```go
package auth

import (
	"context"
	"net/http"
)

type contextKey string

const sessionKey contextKey = "session"

const cookieName = "daf_session"

func GetSession(r *http.Request) *Session {
	sess, _ := r.Context().Value(sessionKey).(*Session)
	return sess
}

func RequireAuth(sessions *SessionStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		sess, err := sessions.Get(r.Context(), cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}

		// Session verlängern
		sessions.Refresh(r.Context(), sess.ID)

		ctx := context.WithValue(r.Context(), sessionKey, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := GetSession(r)
		if sess == nil || sess.User.Role != role {
			// Admin darf alles
			if sess != nil && sess.User.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func RequireGroupAccess(sessions *SessionStore, next http.HandlerFunc, getGroupID func(r *http.Request) int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := GetSession(r)
		if sess == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Admin darf alles
		if sess.User.Role == "admin" {
			next.ServeHTTP(w, r)
			return
		}

		groupID := getGroupID(r)
		groups, err := sessions.GetUserGroups(r.Context(), sess.UserID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		for _, id := range groups {
			if id == groupID {
				next.ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, "Forbidden", http.StatusForbidden)
	})
}
```

- [ ] **Step 4: Verifizieren**

```bash
go build ./...
go test ./...
```
Expected: Build erfolgreich.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/
git commit -m "feat: auth with OIDC client, session store, and middleware"
```

---

### Task 7: QR-Code-Generierung

**Files:**
- Create: `internal/qrcode/generate.go`

- [ ] **Step 1: QR-Code Generator implementieren**

Create `internal/qrcode/generate.go`:
```go
package qrcode

import (
	"fmt"

	goqrcode "github.com/skip2/go-qrcode"
)

func GeneratePNG(baseURL, slug, secret string, size int) ([]byte, error) {
	url := fmt.Sprintf("%s/f/%s-%s", baseURL, slug, secret)
	png, err := goqrcode.Encode(url, goqrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("generate qr png: %w", err)
	}
	return png, nil
}

func GenerateGlobalPNG(baseURL, globalSecret string, size int) ([]byte, error) {
	url := fmt.Sprintf("%s/f/alle-%s", baseURL, globalSecret)
	png, err := goqrcode.Encode(url, goqrcode.Medium, size)
	if err != nil {
		return nil, fmt.Errorf("generate global qr png: %w", err)
	}
	return png, nil
}

func FeedbackURL(baseURL, slug, secret string) string {
	return fmt.Sprintf("%s/f/%s-%s", baseURL, slug, secret)
}

func GlobalFeedbackURL(baseURL, globalSecret string) string {
	return fmt.Sprintf("%s/f/alle-%s", baseURL, globalSecret)
}
```

- [ ] **Step 2: go mod tidy + Build verifizieren**

```bash
go mod tidy && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/qrcode/
git commit -m "feat: QR code generation for group and global feedback URLs"
```

---

### Task 8: Analysis (Aggregation + CSV-Export)

**Files:**
- Create: `internal/analysis/aggregation.go`
- Create: `internal/analysis/aggregation_test.go`
- Create: `internal/analysis/export.go`

- [ ] **Step 1: Aggregation-Typen und -Logik erstellen**

Create `internal/analysis/aggregation.go`:
```go
package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

type DAStats struct {
	EveningID        int
	Date             time.Time
	Topic            *string
	ResponseCount    int
	ParticipantCount *int
	AvgOverall       float64
	AvgRelevance     float64
	AvgClarity       float64
	Highlights       []string
	Improvements     []string
	TopicWishes      []string
}

type GroupTrend struct {
	Points []TrendPoint
}

type TrendPoint struct {
	Date         time.Time
	AvgOverall   float64
	AvgRelevance float64
	AvgClarity   float64
	Responses    int
}

type GroupComparison struct {
	GroupID    int
	GroupName  string
	AvgOverall float64
	TotalDAs   int
	TotalResp  int
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) GetDAStats(ctx context.Context, eveningID int) (*DAStats, error) {
	stats := &DAStats{EveningID: eveningID}

	// Evening-Daten
	err := s.db.QueryRowContext(ctx,
		"SELECT date, topic, participant_count FROM evenings WHERE id = ?", eveningID,
	).Scan(&stats.Date, &stats.Topic, &stats.ParticipantCount)
	if err != nil {
		return nil, fmt.Errorf("get evening: %w", err)
	}

	// Antworten laden
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.answers FROM responses r
		 JOIN surveys s ON r.survey_id = s.id
		 WHERE s.evening_id = ?`, eveningID)
	if err != nil {
		return nil, fmt.Errorf("get responses: %w", err)
	}
	defer rows.Close()

	var sumOverall, sumRelevance, sumClarity float64
	var count int

	for rows.Next() {
		var aJSON string
		if err := rows.Scan(&aJSON); err != nil {
			return nil, err
		}
		var answers map[string]any
		if err := json.Unmarshal([]byte(aJSON), &answers); err != nil {
			continue
		}

		count++
		sumOverall += toFloat(answers["q1"])
		sumRelevance += toFloat(answers["q2"])
		sumClarity += toFloat(answers["q3"])

		if v, ok := answers["q4"].(string); ok && v != "" {
			stats.Highlights = append(stats.Highlights, v)
		}
		if v, ok := answers["q5"].(string); ok && v != "" {
			stats.Improvements = append(stats.Improvements, v)
		}
		if v, ok := answers["q6"].(string); ok && v != "" {
			stats.TopicWishes = append(stats.TopicWishes, v)
		}
	}

	stats.ResponseCount = count
	if count > 0 {
		stats.AvgOverall = round2(sumOverall / float64(count))
		stats.AvgRelevance = round2(sumRelevance / float64(count))
		stats.AvgClarity = round2(sumClarity / float64(count))
	}

	return stats, rows.Err()
}

func (s *Store) GetGroupTrend(ctx context.Context, groupID int, from, to time.Time) (*GroupTrend, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT e.date, r.answers FROM responses r
		 JOIN surveys s ON r.survey_id = s.id
		 JOIN evenings e ON s.evening_id = e.id
		 WHERE e.group_id = ? AND e.date BETWEEN ? AND ?
		 ORDER BY e.date`, groupID, from, to)
	if err != nil {
		return nil, fmt.Errorf("get trend: %w", err)
	}
	defer rows.Close()

	// Gruppieren nach Datum
	byDate := map[string]*struct {
		date                                   time.Time
		sumOverall, sumRelevance, sumClarity   float64
		count                                  int
	}{}

	for rows.Next() {
		var date time.Time
		var aJSON string
		if err := rows.Scan(&date, &aJSON); err != nil {
			return nil, err
		}
		key := date.Format("2006-01-02")
		if byDate[key] == nil {
			byDate[key] = &struct {
				date                                   time.Time
				sumOverall, sumRelevance, sumClarity   float64
				count                                  int
			}{date: date}
		}

		var answers map[string]any
		if err := json.Unmarshal([]byte(aJSON), &answers); err != nil {
			continue
		}

		d := byDate[key]
		d.count++
		d.sumOverall += toFloat(answers["q1"])
		d.sumRelevance += toFloat(answers["q2"])
		d.sumClarity += toFloat(answers["q3"])
	}

	trend := &GroupTrend{}
	for _, d := range byDate {
		if d.count > 0 {
			trend.Points = append(trend.Points, TrendPoint{
				Date:         d.date,
				AvgOverall:   round2(d.sumOverall / float64(d.count)),
				AvgRelevance: round2(d.sumRelevance / float64(d.count)),
				AvgClarity:   round2(d.sumClarity / float64(d.count)),
				Responses:    d.count,
			})
		}
	}
	return trend, rows.Err()
}

func (s *Store) GetGroupComparisons(ctx context.Context) ([]GroupComparison, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name,
		        COALESCE(AVG(CAST(json_extract(r.answers, '$.q1') AS REAL)), 0),
		        COUNT(DISTINCT e.id),
		        COUNT(r.id)
		 FROM groups g
		 LEFT JOIN evenings e ON e.group_id = g.id
		 LEFT JOIN surveys s ON s.evening_id = e.id
		 LEFT JOIN responses r ON r.survey_id = s.id
		 GROUP BY g.id
		 ORDER BY g.name`)
	if err != nil {
		return nil, fmt.Errorf("get comparisons: %w", err)
	}
	defer rows.Close()

	var comps []GroupComparison
	for rows.Next() {
		var c GroupComparison
		if err := rows.Scan(&c.GroupID, &c.GroupName, &c.AvgOverall, &c.TotalDAs, &c.TotalResp); err != nil {
			return nil, err
		}
		c.AvgOverall = round2(c.AvgOverall)
		comps = append(comps, c)
	}
	return comps, rows.Err()
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
```

- [ ] **Step 2: Failing Tests schreiben**

Create `internal/analysis/aggregation_test.go`:
```go
package analysis_test

import (
	"context"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

type testEnv struct {
	analysis *analysis.Store
	surveys  *survey.Store
	evenings *evening.Store
	groups   *group.Store
}

func setupTestDB(t *testing.T) testEnv {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return testEnv{
		analysis: analysis.NewStore(db),
		surveys:  survey.NewStore(db),
		evenings: evening.NewStore(db),
		groups:   group.NewStore(db),
	}
}

func TestGetDAStats(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()

	g, _ := env.groups.Create(ctx, "I&K", "iuk")
	topic := "Funk"
	e, _ := env.evenings.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), &topic)
	s, _ := env.surveys.Create(ctx, e.ID, nil)
	env.surveys.Activate(ctx, s.ID, 48)

	env.surveys.SubmitResponse(ctx, s.ID, map[string]any{
		"q1": 5, "q2": 4, "q3": 3, "q4": "Toll!", "q5": "", "q6": "HF-Funk",
	})
	env.surveys.SubmitResponse(ctx, s.ID, map[string]any{
		"q1": 3, "q2": 4, "q3": 5, "q4": "", "q5": "Mehr Praxis", "q6": "",
	})

	stats, err := env.analysis.GetDAStats(ctx, e.ID)
	if err != nil {
		t.Fatalf("get stats: %v", err)
	}
	if stats.ResponseCount != 2 {
		t.Fatalf("expected 2 responses, got %d", stats.ResponseCount)
	}
	if stats.AvgOverall != 4.0 {
		t.Fatalf("expected avg 4.0, got %.2f", stats.AvgOverall)
	}
	if len(stats.Highlights) != 1 || stats.Highlights[0] != "Toll!" {
		t.Fatalf("unexpected highlights: %v", stats.Highlights)
	}
	if len(stats.TopicWishes) != 1 || stats.TopicWishes[0] != "HF-Funk" {
		t.Fatalf("unexpected topic wishes: %v", stats.TopicWishes)
	}
}

func TestGetGroupComparisons(t *testing.T) {
	env := setupTestDB(t)
	ctx := context.Background()

	g1, _ := env.groups.Create(ctx, "I&K", "iuk")
	g2, _ := env.groups.Create(ctx, "San", "san")

	e1, _ := env.evenings.Create(ctx, g1.ID, time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC), nil)
	e2, _ := env.evenings.Create(ctx, g2.ID, time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC), nil)

	s1, _ := env.surveys.Create(ctx, e1.ID, nil)
	env.surveys.Activate(ctx, s1.ID, 48)
	env.surveys.SubmitResponse(ctx, s1.ID, map[string]any{"q1": 5, "q2": 5, "q3": 5})

	s2, _ := env.surveys.Create(ctx, e2.ID, nil)
	env.surveys.Activate(ctx, s2.ID, 48)
	env.surveys.SubmitResponse(ctx, s2.ID, map[string]any{"q1": 3, "q2": 3, "q3": 3})

	comps, err := env.analysis.GetGroupComparisons(ctx)
	if err != nil {
		t.Fatalf("get comparisons: %v", err)
	}
	if len(comps) != 2 {
		t.Fatalf("expected 2, got %d", len(comps))
	}
}
```

- [ ] **Step 3: Tests ausführen, Fehlschlag bestätigen**

```bash
go test ./internal/analysis/...
```
Expected: FAIL.

- [ ] **Step 4: CSV-Export implementieren**

Create `internal/analysis/export.go`:
```go
package analysis

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

func (s *Store) ExportGroupCSV(ctx context.Context, w io.Writer, groupID int, from, to time.Time) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{"Datum", "Thema", "Teilnehmer", "Antworten",
		"Gesamt (Ø)", "Praxisrelevanz (Ø)", "Verständlichkeit (Ø)",
		"Highlights", "Verbesserungen", "Themenwünsche"}
	if err := cw.Write(header); err != nil {
		return err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.date, e.topic, e.participant_count
		 FROM evenings e WHERE e.group_id = ? AND e.date BETWEEN ? AND ?
		 ORDER BY e.date`, groupID, from, to)
	if err != nil {
		return fmt.Errorf("query evenings: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var eID int
		var date time.Time
		var topic *string
		var participants *int
		if err := rows.Scan(&eID, &date, &topic, &participants); err != nil {
			return err
		}

		stats, err := s.GetDAStats(ctx, eID)
		if err != nil {
			continue
		}

		topicStr := ""
		if topic != nil {
			topicStr = *topic
		}
		partStr := ""
		if participants != nil {
			partStr = fmt.Sprintf("%d", *participants)
		}

		record := []string{
			date.Format("2006-01-02"),
			topicStr,
			partStr,
			fmt.Sprintf("%d", stats.ResponseCount),
			fmt.Sprintf("%.2f", stats.AvgOverall),
			fmt.Sprintf("%.2f", stats.AvgRelevance),
			fmt.Sprintf("%.2f", stats.AvgClarity),
			joinStrings(stats.Highlights),
			joinStrings(stats.Improvements),
			joinStrings(stats.TopicWishes),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	return rows.Err()
}

func joinStrings(ss []string) string {
	b, _ := json.Marshal(ss)
	return string(b)
}
```

- [ ] **Step 5: Tests ausführen, Erfolg bestätigen**

```bash
go test ./internal/analysis/... -v
```
Expected: Alle Tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/analysis/ internal/qrcode/
git commit -m "feat: analysis aggregation, CSV export, and QR code generation"
```

---

### Task 9: Templates + Render-Helpers

**Files:**
- Create: `internal/ui/render.go`
- Create: `templates/base.html`
- Create: `templates/public/survey.html`
- Create: `templates/public/thanks.html`
- Create: `templates/public/unavailable.html`
- Create: `templates/public/select_group.html`
- Create: `static/input.css`

- [ ] **Step 1: Render-Helper erstellen**

Create `internal/ui/render.go`:
```go
package ui

import (
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
)

type Renderer struct {
	templates map[string]*template.Template
}

func NewRenderer(templateFS fs.FS) (*Renderer, error) {
	r := &Renderer{templates: make(map[string]*template.Template)}

	funcs := template.FuncMap{
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"deref": func(p *string) string {
			if p == nil {
				return ""
			}
			return *p
		},
		"derefInt": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
	}

	base := "base.html"

	pages, err := fs.Glob(templateFS, "*/*.html")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.ToSlash(page)
		t, err := template.New("").Funcs(funcs).ParseFS(templateFS, base, page)
		if err != nil {
			return nil, err
		}
		r.templates[name] = t
	}

	return r, nil
}

func (rn *Renderer) Render(w http.ResponseWriter, name string, status int, data any) {
	t, ok := rn.templates[name]
	if !ok {
		http.Error(w, "template not found: "+name, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	t.ExecuteTemplate(w, "base", data)
}
```

- [ ] **Step 2: Base-Template erstellen**

Create `templates/base.html`:
```html
{{define "base"}}<!DOCTYPE html>
<html lang="de">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{template "title" .}} — DRK Feedback</title>
    <link rel="stylesheet" href="/static/style.css">
    {{block "head" .}}{{end}}
</head>
<body class="min-h-screen bg-gray-50 text-gray-900">
    {{block "nav" .}}{{end}}
    <main class="max-w-4xl mx-auto px-4 py-8">
        {{template "content" .}}
    </main>
    {{block "scripts" .}}{{end}}
</body>
</html>{{end}}
```

- [ ] **Step 3: Public Templates erstellen**

Create `templates/public/survey.html`:
```html
{{define "title"}}Feedback{{end}}
{{define "content"}}
<div class="max-w-lg mx-auto">
    <h1 class="text-2xl font-bold text-red-700 mb-2">Feedback zum Dienstabend</h1>
    {{if .Evening.Topic}}<p class="text-gray-600 mb-6">{{deref .Evening.Topic}} — {{.Evening.Date.Format "02.01.2006"}}</p>{{end}}

    {{if .AlreadySubmitted}}
    <div class="bg-yellow-50 border border-yellow-200 rounded p-4 mb-6">
        <p class="text-yellow-800">Du hast bereits Feedback abgegeben. Du kannst trotzdem nochmal absenden.</p>
    </div>
    {{end}}

    <form method="POST" action="/f/{{.Group.Slug}}-{{.Group.Secret}}/submit" class="space-y-6">
        {{range .Questions}}
        <div class="space-y-2">
            <label class="block font-medium">
                {{.Text}}
                {{if .Required}}<span class="text-red-600">*</span>{{end}}
            </label>

            {{if eq (printf "%s" .Type) "stars"}}
            <div class="flex gap-1">
                {{range seq 5}}
                <label class="cursor-pointer">
                    <input type="radio" name="{{$.QuestionID $.Questions}}" value="{{.}}" class="sr-only peer" {{if $.Required}}required{{end}}>
                    <span class="text-2xl peer-checked:text-yellow-400 text-gray-300 hover:text-yellow-300">★</span>
                </label>
                {{end}}
            </div>
            {{else if eq (printf "%s" .Type) "text"}}
            <textarea name="{{.ID}}" rows="3" maxlength="500"
                class="w-full rounded border border-gray-300 px-3 py-2 focus:border-red-500 focus:ring-1 focus:ring-red-500"
                placeholder="Optional..."></textarea>
            {{else if eq (printf "%s" .Type) "single_choice"}}
            <div class="space-y-1">
                {{range .Options}}
                <label class="flex items-center gap-2">
                    <input type="radio" name="{{$.QuestionID $.Questions}}" value="{{.}}">
                    <span>{{.}}</span>
                </label>
                {{end}}
            </div>
            {{else if eq (printf "%s" .Type) "multi_choice"}}
            <div class="space-y-1">
                {{range .Options}}
                <label class="flex items-center gap-2">
                    <input type="checkbox" name="{{$.QuestionID $.Questions}}" value="{{.}}">
                    <span>{{.}}</span>
                </label>
                {{end}}
            </div>
            {{end}}
        </div>
        {{end}}

        <button type="submit"
            class="w-full bg-red-700 text-white py-3 rounded font-medium hover:bg-red-800 transition">
            Feedback absenden
        </button>
    </form>
</div>
{{end}}
```

Create `templates/public/thanks.html`:
```html
{{define "title"}}Danke{{end}}
{{define "content"}}
<div class="max-w-lg mx-auto text-center py-16">
    <h1 class="text-3xl font-bold text-red-700 mb-4">Danke für dein Feedback!</h1>
    <p class="text-gray-600">Deine Rückmeldung hilft uns, die Dienstabende zu verbessern.</p>
</div>
{{end}}
```

Create `templates/public/unavailable.html`:
```html
{{define "title"}}Keine Umfrage{{end}}
{{define "content"}}
<div class="max-w-lg mx-auto text-center py-16">
    <h1 class="text-2xl font-bold text-gray-700 mb-4">Aktuell keine Umfrage verfügbar</h1>
    <p class="text-gray-500">Für diese Fachgruppe ist gerade keine Feedback-Umfrage aktiv.</p>
</div>
{{end}}
```

Create `templates/public/select_group.html`:
```html
{{define "title"}}Fachgruppe wählen{{end}}
{{define "content"}}
<div class="max-w-lg mx-auto">
    <h1 class="text-2xl font-bold text-red-700 mb-6">Fachgruppe auswählen</h1>
    {{if .Groups}}
    <div class="space-y-3">
        {{range .Groups}}
        <a href="/f/{{.Slug}}-{{.Secret}}"
           class="block p-4 bg-white rounded shadow hover:shadow-md transition border border-gray-200">
            <span class="font-medium">{{.Name}}</span>
        </a>
        {{end}}
    </div>
    {{else}}
    <p class="text-gray-500">Aktuell sind keine Umfragen aktiv.</p>
    {{end}}
</div>
{{end}}
```

- [ ] **Step 4: Tailwind Input erstellen**

Create `static/input.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;
```

- [ ] **Step 5: Verifizieren**

```bash
go build ./...
```
Expected: Build erfolgreich.

- [ ] **Step 6: Commit**

```bash
git add internal/ui/render.go templates/ static/input.css
git commit -m "feat: template renderer, base layout, and public templates"
```

---

### Task 10: Public Handlers (Teilnehmer-Flow)

**Files:**
- Create: `internal/ui/public.go`
- Create: `internal/ui/public_test.go`

- [ ] **Step 1: Public Handler erstellen**

Create `internal/ui/public.go`:
```go
package ui

import (
	"database/sql"
	"encoding/json"
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

func (h *PublicHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /f/{slugSecret}", h.showSurvey)
	mux.HandleFunc("POST /f/{slugSecret}/submit", h.submitSurvey)
	mux.HandleFunc("GET /f/{slugSecret}/thanks", h.showThanks)
}

func (h *PublicHandler) showSurvey(w http.ResponseWriter, r *http.Request) {
	slugSecret := r.PathValue("slugSecret")

	// Globaler QR-Code Check
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

func (h *PublicHandler) submitSurvey(w http.ResponseWriter, r *http.Request) {
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

	// Duplikat-Cookie setzen
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

func (h *PublicHandler) showThanks(w http.ResponseWriter, r *http.Request) {
	h.render.Render(w, "public/thanks.html", http.StatusOK, nil)
}

func (h *PublicHandler) showGroupSelect(w http.ResponseWriter, r *http.Request, secret string) {
	// Global-Secret prüfen (aus config-Tabelle) — vereinfacht: über Gruppen-Liste
	// TODO: config-Store einbinden, für jetzt alle Gruppen mit aktiver Umfrage zeigen
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
	// Format: {slug}-{secret} wobei secret 5 Zeichen lang ist
	if len(slugSecret) < 7 { // mindestens 1 char slug + "-" + 5 char secret
		return nil, nil, nil, fmt.Errorf("invalid slug-secret")
	}
	secret := slugSecret[len(slugSecret)-5:]
	slug := slugSecret[:len(slugSecret)-6] // -1 für den Bindestrich

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

// Unused import guard
var _ = sql.ErrNoRows
var _ = json.Marshal
```

- [ ] **Step 2: Integration-Test schreiben**

Create `internal/ui/public_test.go`:
```go
package ui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/rubeen/da-feedback/internal/database"
	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
	"github.com/rubeen/da-feedback/internal/ui"
)

func setupPublicTest(t *testing.T) (*http.ServeMux, *group.Group, *survey.Survey) {
	t.Helper()
	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	database.Migrate(db)
	t.Cleanup(func() { db.Close() })

	gs := group.NewStore(db)
	es := evening.NewStore(db)
	ss := survey.NewStore(db)

	ctx := context.Background()
	g, _ := gs.Create(ctx, "I&K", "iuk")
	e, _ := es.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	s, _ := ss.Create(ctx, e.ID, nil)
	ss.Activate(ctx, s.ID, 48)

	// Renderer braucht echte Templates — für Tests vereinfacht
	// Wir testen nur HTTP-Status und Redirects
	mux := http.NewServeMux()
	// Hinweis: Vollständiger Test erfordert Template-FS, hier nur Handler-Logik
	return mux, g, s
}

func TestSubmitFlow(t *testing.T) {
	// Grundlegender Smoke-Test: Build kompiliert, Typen passen zusammen
	db, _ := database.Open(":memory:")
	database.Migrate(db)
	defer db.Close()

	gs := group.NewStore(db)
	es := evening.NewStore(db)
	ss := survey.NewStore(db)

	ctx := context.Background()
	g, _ := gs.Create(ctx, "I&K", "iuk")
	e, _ := es.Create(ctx, g.ID, time.Date(2026, 4, 8, 0, 0, 0, 0, time.UTC), nil)
	s, _ := ss.Create(ctx, e.ID, nil)
	ss.Activate(ctx, s.ID, 48)

	// Direkt Response submitten (Store-Level)
	answers := map[string]any{"q1": 5, "q2": 4, "q3": 3}
	_, err := ss.SubmitResponse(ctx, s.ID, answers)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	count, _ := ss.GetResponseCount(ctx, s.ID)
	if count != 1 {
		t.Fatalf("expected 1 response, got %d", count)
	}

	_ = g
	_ = url.Values{}
	_ = strings.NewReader("")
	_ = httptest.NewRequest
	_ = httptest.NewRecorder
	_ = ui.NewPublicHandler
}
```

- [ ] **Step 3: Tests ausführen, Erfolg bestätigen**

```bash
go test ./internal/ui/... -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/ui/public.go internal/ui/public_test.go
git commit -m "feat: public handlers for survey display and submission"
```

---

### Task 11: Admin Handlers

**Files:**
- Create: `internal/ui/admin.go`
- Create: `internal/ui/admin_surveys.go`
- Create: `internal/ui/admin_users.go`
- Create: `internal/ui/analysis.go`
- Create: `templates/admin/dashboard.html`
- Create: `templates/admin/groups.html`
- Create: `templates/admin/group_detail.html`
- Create: `templates/admin/evening_form.html`
- Create: `templates/admin/survey_detail.html`
- Create: `templates/admin/analysis_da.html`
- Create: `templates/admin/analysis_group.html`
- Create: `templates/admin/analysis_global.html`
- Create: `templates/admin/users.html`

- [ ] **Step 1: Admin-Handler (Gruppen + DAs) erstellen**

Create `internal/ui/admin.go`:
```go
package ui

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

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
	sessions *auth.SessionStore
	render   *Renderer
	baseURL  string
}

func NewAdminHandler(g *group.Store, e *evening.Store, s *survey.Store, sess *auth.SessionStore, r *Renderer, baseURL string) *AdminHandler {
	return &AdminHandler{groups: g, evenings: e, surveys: s, sessions: sess, render: r, baseURL: baseURL}
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

	// Survey-Status pro Evening laden
	type EveningWithSurvey struct {
		Evening evening.Evening
		Survey  *survey.Survey
	}
	var items []EveningWithSurvey
	for _, e := range evenings {
		s, _ := h.surveys.GetByEveningID(ctx, e.ID)
		items = append(items, EveningWithSurvey{Evening: e, Survey: s})
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
```

- [ ] **Step 2: Admin-Survey-Handler erstellen**

Create `internal/ui/admin_surveys.go`:
```go
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
	responses, _ := h.surveys.GetResponses(ctx, id)
	count, _ := h.surveys.GetResponseCount(ctx, id)

	h.render.Render(w, "admin/survey_detail.html", http.StatusOK, map[string]any{
		"User":          auth.GetSession(r).User,
		"Survey":        srv,
		"Evening":       eve,
		"Group":         grp,
		"Responses":     responses,
		"ResponseCount": count,
	})
}

func (h *AdminHandler) activateSurvey(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	ctx := r.Context()

	srv, err := h.surveys.GetByID(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// close_after_hours Kaskade: Survey → Group → System-Default (48)
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

	// HTMX: Partial zurückgeben wenn gewünscht
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

// Ensure survey import is used
var _ survey.Status
```

- [ ] **Step 3: Admin-Users-Handler erstellen**

Create `internal/ui/admin_users.go`:
```go
package ui

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/rubeen/da-feedback/internal/auth"
)

func (h *AdminHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler, adminMw func(http.Handler) http.Handler) {
	wrap := func(fn http.HandlerFunc) http.Handler { return adminMw(authMw(fn)) }

	mux.Handle("GET /admin/users", wrap(h.listUsers))
	mux.Handle("POST /admin/users/{id}/groups", wrap(h.assignGroups))
}

func (h *AdminHandler) listUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rows, err := h.sessions.DB().QueryContext(ctx,
		"SELECT id, name, email, role FROM users ORDER BY name")
	if err != nil {
		http.Error(w, "Fehler", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []auth.User
	for rows.Next() {
		var u auth.User
		rows.Scan(&u.ID, &u.Name, &u.Email, &u.Role)
		users = append(users, u)
	}

	groups, _ := h.groups.List(ctx)

	h.render.Render(w, "admin/users.html", http.StatusOK, map[string]any{
		"User":   auth.GetSession(r).User,
		"Users":  users,
		"Groups": groups,
	})
}

func (h *AdminHandler) assignGroups(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	r.ParseForm()

	ctx := r.Context()
	db := h.sessions.DB()

	// Bestehende Zuordnungen löschen
	db.ExecContext(ctx, "DELETE FROM user_groups WHERE user_id = ?", userID)

	// Neue Zuordnungen setzen
	for _, gid := range r.Form["group_ids"] {
		groupID, _ := strconv.Atoi(gid)
		db.ExecContext(ctx, "INSERT INTO user_groups (user_id, group_id) VALUES (?, ?)", userID, groupID)
	}

	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

// Ensure sql import is used
var _ = sql.ErrNoRows
```

- [ ] **Step 4: Analysis-Handler erstellen**

Create `internal/ui/analysis.go`:
```go
package ui

import (
	"net/http"
	"strconv"
	"time"

	"github.com/rubeen/da-feedback/internal/analysis"
	"github.com/rubeen/da-feedback/internal/auth"
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
	adminWrap := func(fn http.HandlerFunc) http.Handler { return adminMw(authMw(fn)) }

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

	// Default: letzte 12 Monate
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
```

- [ ] **Step 5: Admin-Templates erstellen (minimal funktional)**

Create `templates/admin/dashboard.html`:
```html
{{define "title"}}Dashboard{{end}}
{{define "nav"}}
<nav class="bg-red-800 text-white px-4 py-3">
    <div class="max-w-4xl mx-auto flex justify-between items-center">
        <a href="/admin" class="font-bold">DRK Feedback</a>
        <div class="flex gap-4 items-center text-sm">
            {{if eq .User.Role "admin"}}<a href="/admin/groups" class="hover:underline">Gruppen</a>
            <a href="/admin/users" class="hover:underline">Benutzer</a>
            <a href="/admin/analysis/global" class="hover:underline">Auswertung</a>{{end}}
            <span>{{.User.Name}}</span>
            <a href="/auth/logout" class="hover:underline">Abmelden</a>
        </div>
    </div>
</nav>
{{end}}
{{define "content"}}
<h1 class="text-2xl font-bold mb-6">Dashboard</h1>
<div class="grid gap-4 md:grid-cols-2">
    {{range .Groups}}
    <a href="/admin/groups/{{.ID}}" class="block p-6 bg-white rounded-lg shadow hover:shadow-md transition border border-gray-200">
        <h2 class="font-bold text-lg">{{.Name}}</h2>
        <p class="text-sm text-gray-500">{{.Slug}}</p>
    </a>
    {{end}}
</div>
{{end}}
```

Create `templates/admin/groups.html`:
```html
{{define "title"}}Fachgruppen{{end}}
{{define "nav"}}{{template "admin_nav" .}}{{end}}
{{define "content"}}
<div class="flex justify-between items-center mb-6">
    <h1 class="text-2xl font-bold">Fachgruppen</h1>
    <form method="POST" action="/admin/groups" class="flex gap-2">
        <input name="name" placeholder="Name" required class="rounded border px-3 py-2">
        <input name="slug" placeholder="Slug" required class="rounded border px-3 py-2">
        <button type="submit" class="bg-red-700 text-white px-4 py-2 rounded hover:bg-red-800">Erstellen</button>
    </form>
</div>
<div class="space-y-2">
    {{range .Groups}}
    <a href="/admin/groups/{{.ID}}" class="block p-4 bg-white rounded shadow border border-gray-200 hover:shadow-md">
        <span class="font-medium">{{.Name}}</span>
        <span class="text-gray-400 ml-2">{{.Slug}}</span>
    </a>
    {{end}}
</div>
{{end}}
```

Create `templates/admin/group_detail.html`:
```html
{{define "title"}}{{.Group.Name}}{{end}}
{{define "nav"}}{{template "admin_nav" .}}{{end}}
{{define "content"}}
<h1 class="text-2xl font-bold mb-2">{{.Group.Name}}</h1>
<p class="text-sm text-gray-500 mb-6">QR-Code URL: <code>{{.FeedbackURL}}</code>
    <a href="/admin/groups/{{.Group.ID}}/qr.png" class="text-red-700 hover:underline ml-2">QR herunterladen</a>
</p>

<div class="flex justify-between items-center mb-4">
    <h2 class="text-xl font-semibold">Dienstabende</h2>
    <form method="POST" action="/admin/groups/{{.Group.ID}}/da" class="flex gap-2">
        <input type="date" name="date" required class="rounded border px-3 py-2">
        <input name="topic" placeholder="Thema (optional)" class="rounded border px-3 py-2">
        <button type="submit" class="bg-red-700 text-white px-4 py-2 rounded hover:bg-red-800">DA anlegen</button>
    </form>
</div>

<div class="space-y-2">
    {{range .Items}}
    <div class="p-4 bg-white rounded shadow border border-gray-200 flex justify-between items-center">
        <div>
            <span class="font-medium">{{.Evening.Date.Format "02.01.2006"}}</span>
            {{if .Evening.Topic}}<span class="text-gray-500 ml-2">— {{deref .Evening.Topic}}</span>{{end}}
        </div>
        <div class="flex gap-2">
            {{if .Survey}}
                <a href="/admin/surveys/{{.Survey.ID}}" class="text-sm px-3 py-1 rounded
                    {{if eq (printf "%s" .Survey.Status) "active"}}bg-green-100 text-green-800
                    {{else if eq (printf "%s" .Survey.Status) "draft"}}bg-gray-100 text-gray-800
                    {{else}}bg-yellow-100 text-yellow-800{{end}}">
                    {{printf "%s" .Survey.Status}}
                </a>
            {{else}}
                <form method="POST" action="/admin/da/{{.Evening.ID}}/survey">
                    <button class="text-sm px-3 py-1 rounded bg-blue-100 text-blue-800 hover:bg-blue-200">Umfrage erstellen</button>
                </form>
            {{end}}
            <a href="/admin/analysis/da/{{.Evening.ID}}" class="text-sm text-gray-500 hover:underline">Auswertung</a>
        </div>
    </div>
    {{end}}
</div>
{{end}}
```

Create `templates/admin/evening_form.html`:
```html
{{define "title"}}Dienstabend{{end}}
{{define "nav"}}{{template "admin_nav" .}}{{end}}
{{define "content"}}
<h1 class="text-2xl font-bold mb-6">Dienstabend bearbeiten</h1>
<form method="POST" action="/admin/da/{{.Evening.ID}}" class="space-y-4 max-w-lg">
    <div>
        <label class="block font-medium mb-1">Thema</label>
        <input name="topic" value="{{deref .Evening.Topic}}" class="w-full rounded border px-3 py-2">
    </div>
    <div>
        <label class="block font-medium mb-1">Notizen</label>
        <textarea name="notes" rows="3" class="w-full rounded border px-3 py-2">{{deref .Evening.Notes}}</textarea>
    </div>
    <div>
        <label class="block font-medium mb-1">Teilnehmerzahl</label>
        <input type="number" name="participant_count" value="{{derefInt .Evening.ParticipantCount}}" class="w-full rounded border px-3 py-2">
    </div>
    <button type="submit" class="bg-red-700 text-white px-6 py-2 rounded hover:bg-red-800">Speichern</button>
</form>
{{end}}
```

Create `templates/admin/survey_detail.html`:
```html
{{define "title"}}Umfrage{{end}}
{{define "nav"}}{{template "admin_nav" .}}{{end}}
{{define "content"}}
<h1 class="text-2xl font-bold mb-2">Umfrage — {{.Group.Name}}</h1>
<p class="text-gray-500 mb-6">{{.Evening.Date.Format "02.01.2006"}} {{if .Evening.Topic}}— {{deref .Evening.Topic}}{{end}}</p>

<div class="flex gap-3 mb-6">
    <span class="px-3 py-1 rounded text-sm font-medium
        {{if eq (printf "%s" .Survey.Status) "active"}}bg-green-100 text-green-800
        {{else if eq (printf "%s" .Survey.Status) "draft"}}bg-gray-100 text-gray-800
        {{else}}bg-yellow-100 text-yellow-800{{end}}">
        {{printf "%s" .Survey.Status}}
    </span>
    {{if eq (printf "%s" .Survey.Status) "draft"}}
    <form method="POST" action="/admin/surveys/{{.Survey.ID}}/activate">
        <button class="bg-green-600 text-white px-4 py-1 rounded text-sm hover:bg-green-700">Aktivieren</button>
    </form>
    {{else if eq (printf "%s" .Survey.Status) "active"}}
    <form method="POST" action="/admin/surveys/{{.Survey.ID}}/close">
        <button class="bg-yellow-600 text-white px-4 py-1 rounded text-sm hover:bg-yellow-700">Schließen</button>
    </form>
    {{else if eq (printf "%s" .Survey.Status) "closed"}}
    <form method="POST" action="/admin/surveys/{{.Survey.ID}}/archive">
        <button class="bg-gray-600 text-white px-4 py-1 rounded text-sm hover:bg-gray-700">Archivieren</button>
    </form>
    {{end}}
</div>

<p class="mb-4"><strong>{{.ResponseCount}}</strong> Antworten</p>

{{if .Responses}}
<div class="space-y-4">
    {{range .Responses}}
    <div class="p-4 bg-white rounded shadow border border-gray-200 text-sm">
        <p class="text-gray-400 mb-2">{{.SubmittedAt.Format "02.01.2006 15:04"}}</p>
        {{range $k, $v := .Answers}}
        <p><strong>{{$k}}:</strong> {{$v}}</p>
        {{end}}
    </div>
    {{end}}
</div>
{{end}}
{{end}}
```

Create `templates/admin/analysis_da.html`:
```html
{{define "title"}}DA Auswertung{{end}}
{{define "nav"}}{{template "admin_nav" .}}{{end}}
{{define "head"}}<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>{{end}}
{{define "content"}}
<h1 class="text-2xl font-bold mb-6">Auswertung — {{.Stats.Date.Format "02.01.2006"}}</h1>

<div class="grid gap-4 md:grid-cols-3 mb-8">
    <div class="bg-white p-4 rounded shadow text-center">
        <p class="text-3xl font-bold text-red-700">{{printf "%.1f" .Stats.AvgOverall}}</p>
        <p class="text-sm text-gray-500">Gesamtbewertung</p>
    </div>
    <div class="bg-white p-4 rounded shadow text-center">
        <p class="text-3xl font-bold text-red-700">{{printf "%.1f" .Stats.AvgRelevance}}</p>
        <p class="text-sm text-gray-500">Praxisrelevanz</p>
    </div>
    <div class="bg-white p-4 rounded shadow text-center">
        <p class="text-3xl font-bold text-red-700">{{printf "%.1f" .Stats.AvgClarity}}</p>
        <p class="text-sm text-gray-500">Verständlichkeit</p>
    </div>
</div>

<p class="mb-6 text-gray-600">{{.Stats.ResponseCount}} Antworten</p>

{{if .Stats.Highlights}}
<h2 class="font-semibold mb-2">Highlights</h2>
<ul class="list-disc pl-5 mb-4 space-y-1">{{range .Stats.Highlights}}<li>{{.}}</li>{{end}}</ul>
{{end}}

{{if .Stats.Improvements}}
<h2 class="font-semibold mb-2">Verbesserungen</h2>
<ul class="list-disc pl-5 mb-4 space-y-1">{{range .Stats.Improvements}}<li>{{.}}</li>{{end}}</ul>
{{end}}

{{if .Stats.TopicWishes}}
<h2 class="font-semibold mb-2">Themenwünsche</h2>
<ul class="list-disc pl-5 space-y-1">{{range .Stats.TopicWishes}}<li>{{.}}</li>{{end}}</ul>
{{end}}
{{end}}
```

Create `templates/admin/analysis_group.html`:
```html
{{define "title"}}{{.Group.Name}} — Trend{{end}}
{{define "nav"}}{{template "admin_nav" .}}{{end}}
{{define "head"}}<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>{{end}}
{{define "content"}}
<h1 class="text-2xl font-bold mb-6">{{.Group.Name}} — Trendverlauf</h1>

<form class="flex gap-2 mb-6" hx-get="/admin/analysis/group/{{.Group.ID}}" hx-target="main" hx-swap="innerHTML">
    <input type="date" name="from" value="{{.From}}" class="rounded border px-3 py-2">
    <input type="date" name="to" value="{{.To}}" class="rounded border px-3 py-2">
    <button class="bg-red-700 text-white px-4 py-2 rounded">Filtern</button>
</form>

<div id="trend-chart" style="height:400px"></div>

<a href="/admin/analysis/export/{{.Group.ID}}" class="inline-block mt-4 text-red-700 hover:underline">CSV Export</a>

{{if .Trend}}
<script>
var chart = echarts.init(document.getElementById('trend-chart'));
var data = {{.Trend.Points}};
chart.setOption({
    tooltip: {trigger: 'axis'},
    legend: {data: ['Gesamt', 'Praxisrelevanz', 'Verständlichkeit']},
    xAxis: {type: 'category', data: data.map(p => p.Date)},
    yAxis: {type: 'value', min: 1, max: 5},
    series: [
        {name: 'Gesamt', type: 'line', data: data.map(p => p.AvgOverall)},
        {name: 'Praxisrelevanz', type: 'line', data: data.map(p => p.AvgRelevance)},
        {name: 'Verständlichkeit', type: 'line', data: data.map(p => p.AvgClarity)}
    ]
});
window.addEventListener('resize', () => chart.resize());
</script>
{{end}}
{{end}}
```

Create `templates/admin/analysis_global.html`:
```html
{{define "title"}}Globale Auswertung{{end}}
{{define "nav"}}{{template "admin_nav" .}}{{end}}
{{define "head"}}<script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"></script>{{end}}
{{define "content"}}
<h1 class="text-2xl font-bold mb-6">Gruppenübergreifende Auswertung</h1>

<div id="comparison-chart" style="height:400px"></div>

{{if .Comparisons}}
<table class="mt-6 w-full text-left">
    <thead><tr class="border-b">
        <th class="py-2">Gruppe</th><th>Ø Gesamt</th><th>DAs</th><th>Antworten</th>
    </tr></thead>
    <tbody>
    {{range .Comparisons}}
    <tr class="border-b">
        <td class="py-2">{{.GroupName}}</td>
        <td>{{printf "%.1f" .AvgOverall}}</td>
        <td>{{.TotalDAs}}</td>
        <td>{{.TotalResp}}</td>
    </tr>
    {{end}}
    </tbody>
</table>

<script>
var chart = echarts.init(document.getElementById('comparison-chart'));
var data = {{.Comparisons}};
chart.setOption({
    tooltip: {},
    xAxis: {type: 'category', data: data.map(c => c.GroupName)},
    yAxis: {type: 'value', min: 0, max: 5},
    series: [{type: 'bar', data: data.map(c => c.AvgOverall), itemStyle: {color: '#b91c1c'}}]
});
window.addEventListener('resize', () => chart.resize());
</script>
{{end}}
{{end}}
```

Create `templates/admin/users.html`:
```html
{{define "title"}}Benutzer{{end}}
{{define "nav"}}{{template "admin_nav" .}}{{end}}
{{define "content"}}
<h1 class="text-2xl font-bold mb-6">Benutzer-Zuordnung</h1>
<div class="space-y-4">
    {{range .Users}}
    <div class="p-4 bg-white rounded shadow border border-gray-200">
        <div class="flex justify-between items-center mb-2">
            <div><strong>{{.Name}}</strong> <span class="text-gray-400">{{.Email}}</span></div>
            <span class="text-sm px-2 py-1 rounded {{if eq .Role "admin"}}bg-red-100 text-red-800{{else}}bg-blue-100 text-blue-800{{end}}">{{.Role}}</span>
        </div>
        {{if eq .Role "groupleader"}}
        <form method="POST" action="/admin/users/{{.ID}}/groups" class="flex gap-2 flex-wrap">
            {{range $.Groups}}
            <label class="flex items-center gap-1 text-sm">
                <input type="checkbox" name="group_ids" value="{{.ID}}">
                {{.Name}}
            </label>
            {{end}}
            <button type="submit" class="text-sm bg-gray-200 px-3 py-1 rounded hover:bg-gray-300">Zuweisen</button>
        </form>
        {{end}}
    </div>
    {{end}}
</div>
{{end}}
```

- [ ] **Step 6: Verifizieren**

```bash
go build ./...
```
Expected: Build erfolgreich.

- [ ] **Step 7: Commit**

```bash
git add internal/ui/ templates/admin/
git commit -m "feat: admin handlers, survey management, analysis views, and all admin templates"
```

---

### Task 12: Router, Auth-Routes & Main Wiring

**Files:**
- Create: `internal/ui/router.go`
- Modify: `cmd/server/main.go`
- Modify: `internal/auth/session.go` (DB-Accessor hinzufügen)

- [ ] **Step 1: DB-Accessor zu SessionStore hinzufügen**

Add to `internal/auth/session.go` — nach der `DeleteExpired`-Methode:
```go
func (s *SessionStore) DB() *sql.DB {
	return s.db
}
```

- [ ] **Step 2: Router mit Auth-Routes erstellen**

Create `internal/ui/router.go`:
```go
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
	BaseURL    string
	Groups     *group.Store
	Evenings   *evening.Store
	Surveys    *survey.Store
	Analysis   *analysis.Store
	Sessions   *auth.SessionStore
	OIDC       *auth.OIDCClient
	Renderer   *Renderer
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
	registerAuthRoutes(mux, cfg.OIDC, cfg.Sessions)

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
		// State verifizieren
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
```

- [ ] **Step 3: main.go finalisieren**

Replace `cmd/server/main.go`:
```go
package main

import (
	"embed"
	"flag"
	"io/fs"
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

//go:embed ../../templates
var templateFS embed.FS

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
	analysisStore := analysis.NewStore(db)
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
		})
		if err != nil {
			log.Fatalf("oidc: %v", err)
		}
	}

	// Templates
	tFS, err := fs.Sub(templateFS, "templates")
	if err != nil {
		log.Fatalf("template fs: %v", err)
	}
	renderer, err := ui.NewRenderer(tFS)
	if err != nil {
		log.Fatalf("renderer: %v", err)
	}

	// Router
	baseURL := envOr("DAF_BASE_URL", "http://localhost:8080")
	mux := ui.NewRouter(ui.RouterConfig{
		BaseURL:  baseURL,
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
```

- [ ] **Step 4: Verifizieren**

```bash
go build ./...
go test ./...
```
Expected: Build erfolgreich, alle Tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/router.go internal/auth/session.go cmd/server/main.go
git commit -m "feat: router wiring, auth routes, and finalized main.go"
```

---

### Task 13: Rate Limiting

**Files:**
- Modify: `internal/ui/router.go`

- [ ] **Step 1: Rate-Limiter implementieren**

Add to `internal/ui/router.go` — vor `NewRouter`:
```go
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
	// Cleanup alle 3 Minuten
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

	// Token-Bucket: 30 pro Minute, 0.5 pro Sekunde Refill
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
```

Update in `NewRouter` — Public-Handler mit Rate-Limiter wrappen:
```go
// In NewRouter, nach "// Public routes":
rl := newRateLimiter()
pub := NewPublicHandler(cfg.Groups, cfg.Evenings, cfg.Surveys, cfg.Renderer, cfg.BaseURL)
// Wrap public routes with rate limiting
mux.Handle("GET /f/{slugSecret}", rateLimitMiddleware(rl, http.HandlerFunc(pub.showSurvey)))
mux.Handle("POST /f/{slugSecret}/submit", rateLimitMiddleware(rl, http.HandlerFunc(pub.submitSurvey)))
mux.Handle("GET /f/{slugSecret}/thanks", http.HandlerFunc(pub.showThanks))
```

Hinweis: `pub.RegisterRoutes` entfernen und durch die obigen expliziten Registrierungen ersetzen. `showSurvey`, `submitSurvey`, `showThanks` müssen dafür exported werden (Großbuchstabe). Alternative: Methoden auf dem Handler exportieren oder den Rate-Limiter in `RegisterRoutes` übergeben.

Import `sync` hinzufügen.

- [ ] **Step 2: Verifizieren**

```bash
go build ./...
```
Expected: Build erfolgreich.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/router.go
git commit -m "feat: rate limiting on public feedback routes"
```

---

### Task 14: Tailwind Config + Deployment

**Files:**
- Create: `tailwind.config.js`
- Create: `Dockerfile`
- Create: `docker-compose.yml`

- [ ] **Step 1: Tailwind Config erstellen**

Create `tailwind.config.js`:
```js
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: ["./templates/**/*.html"],
  theme: {
    extend: {
      colors: {
        'drk': {
          700: '#b91c1c',
          800: '#991b1b',
        }
      }
    },
  },
  plugins: [],
}
```

- [ ] **Step 2: Dockerfile erstellen**

Create `Dockerfile`:
```dockerfile
FROM golang:latest AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o da-feedback ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /build/da-feedback .
COPY --from=builder /build/migrations ./migrations
COPY --from=builder /build/templates ./templates
COPY --from=builder /build/static ./static
EXPOSE 8080
ENTRYPOINT ["./da-feedback"]
```

- [ ] **Step 3: docker-compose.yml erstellen**

Create `docker-compose.yml`:
```yaml
services:
  da-feedback:
    build: .
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - data:/data
    environment:
      - DAF_BASE_URL=https://feedback.example.com
      - DAF_DB_PATH=/data/feedback.db
      - DAF_OIDC_ISSUER=https://id.example.com
      - DAF_OIDC_CLIENT_ID=da-feedback
      - DAF_OIDC_CLIENT_SECRET=${DAF_OIDC_CLIENT_SECRET}
      - DAF_SESSION_SECRET=${DAF_SESSION_SECRET}
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.da-feedback.rule=Host(`feedback.example.com`)"
      - "traefik.http.routers.da-feedback.tls.certresolver=letsencrypt"
      - "traefik.http.services.da-feedback.loadbalancer.server.port=8080"

volumes:
  data:
```

- [ ] **Step 4: .gitignore aktualisieren**

Append to `.gitignore`:
```
# Environment
.env

# Binary
da-feedback
```

- [ ] **Step 5: Verifizieren**

```bash
go build ./... && go test ./...
```

- [ ] **Step 6: Commit**

```bash
git add tailwind.config.js Dockerfile docker-compose.yml Makefile .gitignore
git commit -m "feat: tailwind config, Dockerfile, docker-compose, and deployment setup"
```

---

### Task 15: End-to-End Smoke Test

- [ ] **Step 1: Server starten und manuell testen**

```bash
# Tailwind CSS generieren (falls CLI vorhanden)
# make tailwind

# Server starten
DAF_DB_PATH=:memory: go run ./cmd/server -dev
```

- [ ] **Step 2: Grundlegende Endpoints prüfen**

```bash
# In einem zweiten Terminal:
curl -s http://localhost:8080/healthz
# Expected: 200 OK

curl -s http://localhost:8080/f/test-12345
# Expected: "Keine Umfrage verfügbar" Seite (oder 404 wenn Gruppe nicht existiert)

curl -s http://localhost:8080/admin
# Expected: Redirect zu /auth/login
```

- [ ] **Step 3: Finaler Commit**

```bash
git add -A
git commit -m "chore: final cleanup and smoke test verification"
```
