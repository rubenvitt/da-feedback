# DA-Feedback

Anonymes Feedback-System für Dienstabende. Teilnehmer scannen einen QR-Code, füllen eine zeitlich begrenzte Umfrage aus. Admins verwalten Gruppen, Abende und Umfragen über ein geschütztes Dashboard mit Auswertungen.

## Features

- QR-Code-basierter Zugang zu anonymen Umfragen
- Automatische Umfrage-Lifecycle (Entwurf → Aktiv → Geschlossen → Archiviert)
- Admin-Dashboard mit Gruppen-, DA- und Globalauswertung (ECharts)
- CSV-Export der Ergebnisse
- OIDC-Authentifizierung (PocketID)
- SQLite-Datenbank, keine externe Infrastruktur nötig

## Voraussetzungen

- Go 1.22+
- [Tailwind CSS CLI](https://tailwindcss.com/blog/standalone-cli) (als `tailwindcss` im Projektroot)

## Starten

```bash
# CSS generieren
./tailwindcss -i static/input.css -o static/style.css --watch

# Server starten (Dev-Modus mit Auto-Login)
go run ./cmd/server -dev
```

Der Server läuft auf `http://localhost:8080`. Im Dev-Modus wird unter `/auth/login` automatisch eine Admin-Session erstellt.

## Konfiguration

| Variable | Standard | Beschreibung |
|---|---|---|
| `DAF_DB_PATH` | `feedback.db` | Pfad zur SQLite-Datenbank |
| `DAF_ADDR` | `:8080` | Server-Adresse |
| `DAF_BASE_URL` | `http://localhost:8080` | Öffentliche URL (für QR-Codes) |
| `DAF_OIDC_ISSUER` | — | OIDC Issuer URL |
| `DAF_OIDC_CLIENT_ID` | — | OIDC Client ID |
| `DAF_OIDC_CLIENT_SECRET` | — | OIDC Client Secret |

## Migrationen

```bash
go run ./cmd/server -migrate
```

## Docker

```bash
docker build -t da-feedback .
docker run -p 8080:8080 -v ./data:/app/feedback.db da-feedback
```

Das Image wird auch automatisch via GitHub Actions nach `ghcr.io` gepusht (bei Push auf `main` oder Version-Tags).

## Projektstruktur

```
cmd/server/      Einstiegspunkt
internal/
  auth/          OIDC, Sessions, Middleware
  group/         Fachgruppen-Verwaltung
  evening/       Dienstabende
  survey/        Umfragen mit Lifecycle
  analysis/      Aggregation, Trends, CSV-Export
  qrcode/        QR-Code-Generierung
  ui/            HTTP-Handler und Templates
templates/       Go HTML-Templates
static/          CSS
migrations/      SQL-Migrationen (goose)
```
