# DRK Dienstabend-Feedback-System — Technische Spec

## 1. Überblick

System zur anonymen Feedback-Erfassung nach Dienstabenden (DA) der DRK-Fachdienstgruppen. Teilnehmer scannen einen statischen QR-Code, füllen eine zeitlich begrenzte Umfrage aus. Admins und Gruppenleiter verwalten und analysieren Ergebnisse über ein geschütztes Dashboard.

## 2. Tech-Stack

| Komponente | Technologie |
|------------|-------------|
| Backend | Go |
| HTTP | net/http (stdlib) |
| Frontend (Teilnehmer) | Go Templates, server-rendered, kein JS nötig |
| Frontend (Admin) | Go Templates + HTMX |
| CSS | Tailwind CSS (Standalone CLI, kein Node.js) |
| Charts | Apache ECharts |
| Datenbank | SQLite (modernc.org/sqlite, pure Go) |
| Migrationen | goose (embedded SQL) |
| Auth | PocketID (OIDC) |
| Deployment | Docker (golang:latest → alpine:latest), Litestream |
| Reverse Proxy | Traefik (extern, bestehend) |

## 3. Projektstruktur

```
da-feedback/
├── cmd/server/main.go           # Einstiegspunkt, Dependency Wiring
├── internal/
│   ├── auth/                    # PocketID OIDC, Session-Management, Middleware
│   │   ├── oidc.go              # OIDC Client, Token-Exchange, Claims
│   │   ├── session.go           # Session-Store (SQLite-backed)
│   │   └── middleware.go        # RequireAuth, RequireRole, RequireGroupAccess
│   ├── group/                   # Fachgruppen
│   │   ├── model.go             # Group Struct
│   │   └── store.go             # CRUD, Slug+Secret Verwaltung
│   ├── evening/                 # Dienstabende
│   │   ├── model.go             # Evening Struct
│   │   └── store.go             # CRUD, Teilnehmerzahl
│   ├── survey/                  # Umfragen + Antworten
│   │   ├── model.go             # Survey, Response, Status, QuestionType
│   │   ├── store.go             # CRUD, Statusübergänge
│   │   ├── lifecycle.go         # Auto-Close Goroutine, Aktivierungslogik
│   │   └── questions.go         # Standard-Fragenblock, Fragetyp-Definitionen
│   ├── analysis/                # Auswertungen
│   │   ├── aggregation.go       # Durchschnitte, Trends, Vergleiche
│   │   └── export.go            # CSV-Export
│   ├── qrcode/                  # QR-Code Generierung
│   │   └── generate.go          # PNG/SVG via skip2/go-qrcode
│   ├── ui/                      # HTTP-Schicht
│   │   ├── router.go            # Route-Registrierung
│   │   ├── public.go            # Handler: Umfrage, Submit, Danke
│   │   ├── admin.go             # Handler: Dashboard, CRUD
│   │   ├── analysis.go          # Handler: Auswertungsseiten
│   │   └── render.go            # Template-Helper, Flash-Messages
│   └── database/                # Datenbankverbindung
│       └── db.go                # SQLite-Init, goose-Embedding
├── migrations/                  # SQL-Migrationsdateien (goose)
│   └── 001_initial.sql
├── templates/
│   ├── base.html                # Layout (HTMX, ECharts, Tailwind)
│   ├── public/
│   │   ├── survey.html          # Umfrage-Formular
│   │   ├── thanks.html          # Bestätigungsseite
│   │   ├── unavailable.html     # Keine aktive Umfrage
│   │   └── select-group.html   # Gruppenauswahl (globaler QR)
│   └── admin/
│       ├── dashboard.html       # Übersicht
│       ├── groups.html          # Fachgruppen-Liste
│       ├── group-detail.html    # Einzelne Fachgruppe + DAs
│       ├── evening-form.html    # DA anlegen/bearbeiten
│       ├── survey-detail.html   # Umfrage verwalten
│       ├── analysis-da.html     # Auswertung einzelner DA
│       ├── analysis-group.html  # Auswertung Fachgruppe
│       ├── analysis-global.html # Gruppenübergreifend
│       └── users.html           # Benutzer-Zuordnung
├── static/
│   ├── input.css                # Tailwind Input
│   └── style.css                # Tailwind Output (generiert)
├── Dockerfile
├── docker-compose.yml
├── Makefile
├── tailwind.config.js
└── .gitignore
```

**Architektur-Prinzip:** Monolith mit Domain-Packages. Jedes Package unter `internal/` enthält Typen (Structs), Store (SQL-Queries) und Logik (Business-Regeln). `internal/ui/` ist eine dünne HTTP-Schicht ohne Business-Code.

**Dependency Flow:** `main.go` erstellt DB-Verbindung → erstellt Stores → erstellt Handler → registriert Routes → startet Server + Auto-Close Goroutine.

## 4. Datenmodell

### 4.1 SQLite-Schema

```sql
-- Systemkonfiguration
CREATE TABLE config (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
-- Initiale Werte: global_qr_secret, default_close_after_hours (48)

-- Fachgruppen
CREATE TABLE groups (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    name              TEXT    NOT NULL,
    slug              TEXT    NOT NULL UNIQUE,
    secret            TEXT    NOT NULL,  -- 5 Zeichen, alphanumerisch
    close_after_hours INTEGER,           -- NULL = Systemstandard
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_groups_slug ON groups(slug);

-- Dienstabende
CREATE TABLE evenings (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    group_id          INTEGER NOT NULL REFERENCES groups(id),
    date              DATE    NOT NULL,
    topic             TEXT,
    notes             TEXT,
    participant_count INTEGER,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_evenings_group_date ON evenings(group_id, date);

-- Umfragen
CREATE TABLE surveys (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    evening_id        INTEGER NOT NULL UNIQUE REFERENCES evenings(id),
    status            TEXT    NOT NULL DEFAULT 'draft'
                      CHECK(status IN ('draft', 'active', 'closed', 'archived')),
    questions         TEXT    NOT NULL DEFAULT '[]',  -- JSON
    close_after_hours INTEGER,                        -- NULL = Gruppen-/Systemstandard
    activated_at      TIMESTAMP,
    closes_at         TIMESTAMP,
    closed_at         TIMESTAMP,
    created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_surveys_status ON surveys(status);

-- Antworten (anonym)
CREATE TABLE responses (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    survey_id    INTEGER   NOT NULL REFERENCES surveys(id),
    answers      TEXT      NOT NULL,  -- JSON
    submitted_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_responses_survey ON responses(survey_id);

-- Benutzer (aus PocketID)
CREATE TABLE users (
    id         TEXT PRIMARY KEY,  -- OIDC sub claim
    name       TEXT NOT NULL,
    email      TEXT NOT NULL,
    role       TEXT NOT NULL CHECK(role IN ('admin', 'groupleader')),
    last_login TIMESTAMP
);

-- Benutzer ↔ Fachgruppen
CREATE TABLE user_groups (
    user_id  TEXT    NOT NULL REFERENCES users(id),
    group_id INTEGER NOT NULL REFERENCES groups(id),
    PRIMARY KEY (user_id, group_id)
);

-- Sessions
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT      NOT NULL REFERENCES users(id),
    data       TEXT,              -- JSON
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
```

### 4.2 JSON-Strukturen

**Survey.questions:**
```json
[
    {
        "id": "q1",
        "type": "stars",
        "text": "Wie bewertest du den heutigen Dienstabend insgesamt?",
        "required": true,
        "standard": true
    },
    {
        "id": "q7",
        "type": "single_choice",
        "text": "Zusatzfrage vom Gruppenleiter",
        "options": ["Option A", "Option B", "Option C"],
        "required": false,
        "standard": false
    }
]
```

**Response.answers:**
```json
{
    "q1": 4,
    "q2": 5,
    "q3": 3,
    "q4": "Die praktische Übung war super",
    "q5": "",
    "q6": "Erste Hilfe Auffrischung",
    "q7": "Option B"
}
```

**Standard-Fragenblock (konstant, immer enthalten):**

| ID | Typ | Frage | Pflicht |
|----|-----|-------|---------|
| q1 | stars | Wie bewertest du den heutigen Dienstabend insgesamt? | Ja |
| q2 | stars | Wie praxisrelevant waren die Inhalte? | Ja |
| q3 | stars | Wie verständlich war die Vermittlung? | Ja |
| q4 | text | Was hat dir besonders gut gefallen? | Nein |
| q5 | text | Was könnte beim nächsten Mal besser laufen? | Nein |
| q6 | text | Welches Thema wünschst du dir für einen kommenden DA? | Nein |

**Zusatz-Fragetypen:** `stars` (1-5), `text` (Freitext), `single_choice`, `multi_choice`

## 5. Authentifizierung & Autorisierung

### 5.1 OIDC-Flow

1. User klickt "Anmelden" → Redirect zu PocketID (`/authorize`)
2. PocketID authentifiziert → Redirect zurück mit Authorization Code
3. Backend tauscht Code gegen Token → liest Claims (sub, name, email, groups)
4. Gruppen-Mapping:
   - `da-feedback-admin` → Rolle `admin`
   - `da-feedback-gl` → Rolle `groupleader`
   - Keine der beiden Gruppen → Zugriff verweigert
5. User wird in `users`-Tabelle angelegt oder aktualisiert
6. Session-Cookie: `httpOnly`, `secure`, `sameSite=lax`, enthält Session-ID
7. Session-Daten in SQLite (`sessions`-Tabelle), TTL 7 Tage, Refresh bei Aktivität

### 5.2 Middleware-Chain

```
Public Routes:   /f/{slug}-{secret}         → kein Auth
Admin Routes:    /admin/*                    → RequireAuth → RequireRole(admin|groupleader)
Gruppen-Routes:  /admin/groups/{id}/*        → RequireAuth → RequireGroupAccess(id)
Admin-Only:      /admin/users, /admin/config → RequireAuth → RequireRole(admin)
```

### 5.3 Autorisierungsregeln

- **Admin:** Vollzugriff auf alle Fachgruppen, Benutzer-Zuordnung, globale Auswertungen, Systemkonfiguration
- **Gruppenleiter:** Zugriff nur auf zugewiesene Fachgruppen (via `user_groups`), kann dort DAs anlegen, Umfragen verwalten, Auswertungen einsehen
- Fachgruppen-Zuordnung wird vom Admin im Dashboard verwaltet

## 6. Teilnehmer-Flow

### 6.1 URL-Routing

```
GET /f/{slug}-{secret}
  1. DB: Fachgruppe mit passendem slug UND secret suchen
  2. Nicht gefunden → 404
  3. DB: Aktive Umfrage der Fachgruppe suchen (status='active')
  4. Keine aktiv → templates/public/unavailable.html rendern
  5. Aktiv → templates/public/survey.html mit Fragen rendern

GET /f/alle-{global_secret}
  1. DB: global_qr_secret aus config prüfen
  2. Passt nicht → 404
  3. Alle Fachgruppen laden, die eine aktive Umfrage haben
  4. templates/public/select-group.html rendern → Links zu /f/{slug}-{secret}

POST /f/{slug}-{secret}/submit
  1. Fachgruppe + aktive Umfrage auflösen (wie GET)
  2. Formular-Daten validieren (Pflichtfragen ausgefüllt, Sterne 1-5)
  3. Response in DB speichern (answers als JSON)
  4. Cookie setzen: feedback-{survey_id}=submitted (24h TTL)
  5. Redirect zu /f/{slug}-{secret}/thanks (PRG-Pattern)
```

### 6.2 Formular

- Reines HTML `<form>` — funktioniert ohne JavaScript
- Sterne-Rating: 5 Radio-Buttons pro Frage (Progressive Enhancement: JS macht sie zu klickbaren Sternen)
- Freitext: `<textarea>` mit Zeichenlimit (500 Zeichen)
- Zusatzfragen dynamisch aus `survey.questions` JSON gerendert

### 6.3 Duplikat-Schutz

- Cookie-basiert: `feedback-{survey_id}=submitted` mit 24h TTL
- Wenn Cookie vorhanden: Hinweis "Du hast bereits Feedback gegeben" + Option trotzdem abzusenden
- Kein harter Block (anonymes System, Cookies löschbar)

## 7. Admin-Dashboard

### 7.1 Routes

```
GET  /admin                          → Dashboard (Übersicht eigene Gruppen)
GET  /admin/groups                   → Fachgruppen-Liste
POST /admin/groups                   → Fachgruppe erstellen (Admin)
GET  /admin/groups/{id}              → Fachgruppe: DAs, Umfragen, QR-Code
PUT  /admin/groups/{id}              → Fachgruppe bearbeiten (Admin)
DEL  /admin/groups/{id}              → Fachgruppe löschen (Admin)

POST /admin/groups/{id}/da           → Neuen DA anlegen
GET  /admin/da/{id}                  → DA-Detail
PATCH /admin/da/{id}                 → DA aktualisieren (Teilnehmerzahl)

POST /admin/da/{id}/survey           → Umfrage erstellen
GET  /admin/surveys/{id}             → Umfrage-Detail
POST /admin/surveys/{id}/activate    → Umfrage aktivieren
POST /admin/surveys/{id}/close       → Umfrage manuell schließen
POST /admin/surveys/{id}/archive     → Umfrage archivieren

GET  /admin/analysis/da/{id}         → Auswertung einzelner DA
GET  /admin/analysis/group/{id}      → Auswertung Fachgruppe (Trend)
GET  /admin/analysis/global          → Gruppenübergreifend (Admin)
GET  /admin/analysis/export/{id}     → CSV-Export Fachgruppe

GET  /admin/users                    → Benutzer-Zuordnung (Admin)
POST /admin/users/{id}/groups        → Fachgruppen zuweisen
```

### 7.2 HTMX-Interaktionen

- **Umfrage aktivieren:** `hx-post` → Server schließt vorherige aktive Umfrage, aktiviert neue, gibt aktualisiertes Status-Fragment zurück
- **DA-Liste filtern:** `hx-get` mit Query-Params → partielles HTML-Fragment
- **Teilnehmerzahl nachtragen:** `hx-patch` → Inline-Edit mit `hx-swap="outerHTML"`
- **Auswertungs-Zeitraum:** `hx-get` mit Datums-Params → Chart-Container mit neuen Daten

### 7.3 ECharts-Integration

- Chart-Daten werden als JSON in ein `<script>`-Tag im Template geschrieben
- HTMX `htmx:afterSwap`-Event initialisiert neue Charts via `echarts.init()`
- Responsive: `window.addEventListener('resize', () => chart.resize())`

## 8. Auswertungen

### 8.1 Einzelner DA

- Durchschnittswerte pro Skalenfrage (q1, q2, q3)
- Anzahl eingegangener Antworten
- Freitext-Antworten als Liste (q4, q5, q6)
- Vergleich zum Gruppendurchschnitt (Delta-Anzeige)

### 8.2 Fachgruppen-Ebene

- Trendverlauf: Liniendiagramm der Durchschnittsbewertungen über Zeit
- Gesamt-Durchschnitte über alle DAs
- Teilnahmequote: Antworten / Teilnehmerzahl pro DA (wenn Teilnehmerzahl eingetragen)
- Themenwünsche gesammelt (Freitext aus q6)
- Bester/schlechtester DA im gewählten Zeitraum

### 8.3 Global (nur Admin)

- Balkendiagramm: Gruppenvergleich (Durchschnittsbewertungen)
- Gesamtstatistiken: Anzahl DAs, Umfragen, Antworten
- Globale Trends über alle Gruppen
- CSV-Export pro Fachgruppe

## 9. QR-Code-Verwaltung

- Generierung serverseitig via `skip2/go-qrcode`
- Formate: PNG (300×300) und SVG
- Im Admin-Dashboard: QR-Code-Vorschau + Download pro Fachgruppe
- Globaler QR-Code unter Systemeinstellungen
- Secret regenerierbar durch Admin (→ neuer QR-Code nötig)

## 10. Auto-Close

- Background-Goroutine, startet mit dem Server
- Prüft alle 5 Minuten: `SELECT * FROM surveys WHERE status='active' AND closes_at < NOW()`
- Setzt `status='closed'` und `closed_at=NOW()`
- `closes_at` wird bei Aktivierung berechnet: `activated_at + close_after_hours`
- Kaskade: Survey-spezifisch → Gruppen-spezifisch → System-Default (48h)

## 11. Rate-Limiting

- In-Memory Rate-Limiter auf `/f/`-Routes
- 30 Requests/Minute pro IP (Token-Bucket)
- Schutz gegen URL-Raten, nicht gegen DDoS (Traefik-Aufgabe)

## 12. Deployment

### 12.1 Dockerfile

```dockerfile
FROM golang:latest AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o da-feedback ./cmd/server

FROM alpine:latest
RUN apk add --no-cache ca-certificates
COPY --from=builder /build/da-feedback /usr/local/bin/
COPY --from=builder /build/migrations /migrations
COPY --from=builder /build/templates /templates
COPY --from=builder /build/static /static
EXPOSE 8080
ENTRYPOINT ["da-feedback"]
```

### 12.2 Umgebungsvariablen

```
DAF_BASE_URL=https://feedback.example.com
DAF_DB_PATH=/data/feedback.db
DAF_OIDC_ISSUER=https://id.example.com
DAF_OIDC_CLIENT_ID=da-feedback
DAF_OIDC_CLIENT_SECRET=...
DAF_SESSION_SECRET=...
```

### 12.3 Makefile

```makefile
dev:      # Go (Air hot-reload) + Tailwind --watch
build:    # Production Binary + Tailwind --minify
docker:   # Docker Image bauen
migrate:  # goose up manuell
```

### 12.4 Litestream

- SQLite-Backup zu S3-kompatiblem Storage
- Als Entrypoint-Wrapper im Docker-Container
- Automatischer Restore beim Container-Start

## 13. Nicht-funktionale Anforderungen

- Teilnehmer-Seite: < 2s Ladezeit auf mobilem Netz
- Formular funktioniert ohne JavaScript (Progressive Enhancement)
- Responsive: Mobile-first für Teilnehmer, Desktop-optimiert für Admin
- Keine IP-Speicherung, keine Tracking-Cookies
- Einziger Cookie: Session (Admin) und Duplikat-Schutz (Teilnehmer, optional)
