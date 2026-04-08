package survey

import (
	"context"
	"database/sql"
	"encoding/json"
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
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get survey %d: %w", id, ErrNotFound)
		}
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get survey by evening %d: %w", eveningID, ErrNotFound)
		}
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

	// Get group for this survey
	var groupID int
	err = tx.QueryRowContext(ctx,
		`SELECT e.group_id FROM surveys s
		 JOIN evenings e ON s.evening_id = e.id
		 WHERE s.id = ?`, id).Scan(&groupID)
	if err != nil {
		return fmt.Errorf("get group for survey: %w", err)
	}

	// Close all active surveys for this group
	now := time.Now()
	_, err = tx.ExecContext(ctx,
		`UPDATE surveys SET status = 'closed', closed_at = ?
		 WHERE status = 'active' AND evening_id IN (
		     SELECT id FROM evenings WHERE group_id = ?
		 )`, now, groupID)
	if err != nil {
		return fmt.Errorf("close active surveys: %w", err)
	}

	// Activate this survey
	closesAt := now.Add(time.Duration(closeAfterHours) * time.Hour)
	res, err := tx.ExecContext(ctx,
		`UPDATE surveys SET status = 'active', activated_at = ?, closes_at = ?
		 WHERE id = ? AND status = 'draft'`,
		now, closesAt, id)
	if err != nil {
		return fmt.Errorf("activate survey: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("activate survey %d: %w", id, ErrNotFound)
	}

	return tx.Commit()
}

func (s *Store) Close(ctx context.Context, id int) error {
	now := time.Now()
	res, err := s.db.ExecContext(ctx,
		"UPDATE surveys SET status = 'closed', closed_at = ? WHERE id = ?",
		now, id)
	if err != nil {
		return fmt.Errorf("close survey: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("close survey: %w", ErrNotFound)
	}
	return nil
}

func (s *Store) Archive(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE surveys SET status = 'archived' WHERE id = ? AND status = 'closed'", id)
	if err != nil {
		return fmt.Errorf("archive survey: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("archive survey: %w", ErrNotFound)
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("get active survey for group %d: %w", groupID, ErrNotFound)
		}
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

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
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

// CloseExpired closes all active surveys that have passed their closes_at time.
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
