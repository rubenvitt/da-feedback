package evening

import (
	"context"
	"database/sql"
	"errors"
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
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return s.GetByID(ctx, int(id))
}

func (s *Store) GetByID(ctx context.Context, id int) (*Evening, error) {
	e := &Evening{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, group_id, date, topic, notes, participant_count, created_at FROM evenings WHERE id = ?", id,
	).Scan(&e.ID, &e.GroupID, &e.Date, &e.Topic, &e.Notes, &e.ParticipantCount, &e.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get evening %d: %w", id, ErrNotFound)
		}
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
	res, err := s.db.ExecContext(ctx,
		"UPDATE evenings SET participant_count = ? WHERE id = ?", count, id)
	if err != nil {
		return fmt.Errorf("update participant count: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update participant count: %w", ErrNotFound)
	}
	return nil
}

func (s *Store) Update(ctx context.Context, e *Evening) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE evenings SET date = ?, topic = ?, notes = ?, participant_count = ? WHERE id = ?",
		e.Date, e.Topic, e.Notes, e.ParticipantCount, e.ID)
	if err != nil {
		return fmt.Errorf("update evening: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("update evening: %w", ErrNotFound)
	}
	return nil
}

func (s *Store) Delete(ctx context.Context, id int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete evening: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM responses
		 WHERE survey_id IN (SELECT id FROM surveys WHERE evening_id = ?)`, id); err != nil {
		return fmt.Errorf("delete evening responses: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM surveys WHERE evening_id = ?", id); err != nil {
		return fmt.Errorf("delete evening surveys: %w", err)
	}

	res, err := tx.ExecContext(ctx, "DELETE FROM evenings WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete evening: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("delete evening: %w", ErrNotFound)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete evening: %w", err)
	}
	return nil
}
