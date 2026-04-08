package group

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
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

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(ctx, int(id))
}

func (s *Store) GetByID(ctx context.Context, id int) (*Group, error) {
	g := &Group{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, slug, secret, close_after_hours, created_at FROM groups WHERE id = ?", id,
	).Scan(&g.ID, &g.Name, &g.Slug, &g.Secret, &g.CloseAfterHours, &g.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get group %d: %w", id, ErrNotFound)
		}
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get group by slug: %w", ErrNotFound)
		}
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
	res, err := s.db.ExecContext(ctx,
		"UPDATE groups SET name = ?, slug = ?, close_after_hours = ? WHERE id = ?",
		g.Name, g.Slug, g.CloseAfterHours, g.ID)
	if err != nil {
		return fmt.Errorf("update group: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update group: %w", ErrNotFound)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM groups WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete group: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("delete group: %w", ErrNotFound)
	}
	return nil
}

func (s *Store) RegenerateSecret(ctx context.Context, id int) (string, error) {
	secret, err := generateSecret()
	if err != nil {
		return "", fmt.Errorf("generate secret: %w", err)
	}
	res, err := s.db.ExecContext(ctx, "UPDATE groups SET secret = ? WHERE id = ?", secret, id)
	if err != nil {
		return "", fmt.Errorf("update secret: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return "", fmt.Errorf("regenerate secret: %w", ErrNotFound)
	}
	return secret, nil
}
