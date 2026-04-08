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
