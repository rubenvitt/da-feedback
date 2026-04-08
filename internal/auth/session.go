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
	err := s.db.QueryRowContext(ctx,
		`SELECT s.id, s.user_id, s.expires_at, u.name, u.email, u.role
		 FROM sessions s JOIN users u ON s.user_id = u.id
		 WHERE s.id = ? AND s.expires_at > ?`, id, time.Now(),
	).Scan(&sess.ID, &sess.UserID, &sess.ExpiresAt,
		&sess.User.Name, &sess.User.Email, &sess.User.Role)
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

func (s *SessionStore) DB() *sql.DB {
	return s.db
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func UserToJSON(u *User) string {
	b, _ := json.Marshal(u)
	return string(b)
}
