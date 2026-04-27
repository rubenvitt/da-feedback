package analysis

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/rubeen/da-feedback/internal/survey"
)

type RatingAvg struct {
	QuestionID   string  `json:"questionId"`
	QuestionText string  `json:"questionText"`
	Avg          float64 `json:"avg"`
}

type TextResponses struct {
	QuestionID   string
	QuestionText string
	Responses    []string
}

type DAStats struct {
	EveningID        int
	Date             time.Time
	Topic            *string
	ResponseCount    int
	ParticipantCount *int
	AvgOverall       float64         // Durchschnitt aller Bewertungsfragen
	Ratings          []RatingAvg     // Pro Bewertungsfrage
	TextAnswers      []TextResponses // Pro Freitextfrage
}

type GroupTrend struct {
	Points []TrendPoint
}

type TrendPoint struct {
	Date       time.Time   `json:"date"`
	AvgOverall float64     `json:"avgOverall"`
	Ratings    []RatingAvg `json:"ratings"`
	Responses  int         `json:"responses"`
}

type GroupComparison struct {
	GroupID    int         `json:"groupId"`
	GroupName  string      `json:"groupName"`
	AvgOverall float64     `json:"avgOverall"`
	Ratings    []RatingAvg `json:"ratings"`
	TotalDAs   int         `json:"totalDAs"`
	TotalResp  int         `json:"totalResp"`
}

type Store struct {
	db      *sql.DB
	surveys *survey.Store
}

func NewStore(db *sql.DB, surveys *survey.Store) *Store {
	return &Store{db: db, surveys: surveys}
}

func (s *Store) GetDAStats(ctx context.Context, eveningID int) (*DAStats, error) {
	stats := &DAStats{EveningID: eveningID}

	err := s.db.QueryRowContext(ctx,
		"SELECT date, topic, participant_count FROM evenings WHERE id = ?", eveningID,
	).Scan(&stats.Date, &stats.Topic, &stats.ParticipantCount)
	if err != nil {
		return nil, fmt.Errorf("get evening: %w", err)
	}

	// Fragen der Umfrage laden
	var questions []survey.Question
	err = s.db.QueryRowContext(ctx,
		`SELECT s.questions FROM surveys s WHERE s.evening_id = ?`, eveningID,
	).Scan(scanJSON(&questions))
	if err != nil {
		// Fallback: Standard-Fragen verwenden
		questions = survey.StandardQuestions
	}

	// Ratings und Texte nach Fragetyp aufteilen
	var ratingQs, textQs []survey.Question
	for _, q := range questions {
		switch q.Type {
		case survey.TypeSchulnote, survey.TypeStars:
			ratingQs = append(ratingQs, q)
		case survey.TypeText:
			textQs = append(textQs, q)
		}
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT r.answers FROM responses r
		 JOIN surveys s ON r.survey_id = s.id
		 WHERE s.evening_id = ?`, eveningID)
	if err != nil {
		return nil, fmt.Errorf("get responses: %w", err)
	}
	defer rows.Close()

	ratingSums := make(map[string]float64)
	textCollected := make(map[string][]string)
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

		for _, q := range ratingQs {
			ratingSums[q.ID] += toFloat(answers[q.ID])
		}
		for _, q := range textQs {
			if v, ok := answers[q.ID].(string); ok && v != "" {
				textCollected[q.ID] = append(textCollected[q.ID], v)
			}
		}
	}

	stats.ResponseCount = count

	if count > 0 {
		var totalSum float64
		for _, q := range ratingQs {
			avg := round2(ratingSums[q.ID] / float64(count))
			stats.Ratings = append(stats.Ratings, RatingAvg{
				QuestionID:   q.ID,
				QuestionText: q.Text,
				Avg:          avg,
			})
			totalSum += avg
		}
		if len(ratingQs) > 0 {
			stats.AvgOverall = round2(totalSum / float64(len(ratingQs)))
		}
	} else {
		for _, q := range ratingQs {
			stats.Ratings = append(stats.Ratings, RatingAvg{
				QuestionID:   q.ID,
				QuestionText: q.Text,
				Avg:          0,
			})
		}
	}

	for _, q := range textQs {
		stats.TextAnswers = append(stats.TextAnswers, TextResponses{
			QuestionID:   q.ID,
			QuestionText: q.Text,
			Responses:    textCollected[q.ID],
		})
	}

	return stats, rows.Err()
}

func (s *Store) GetGroupTrend(ctx context.Context, groupID int, from, to time.Time) (*GroupTrend, error) {
	// Alle Bewertungsfragen der Gruppe ermitteln (Standard-Fragen als Fallback)
	ratingQs := ratingQuestions(survey.StandardQuestions)

	rows, err := s.db.QueryContext(ctx,
		`SELECT e.date, r.answers FROM responses r
		 JOIN surveys s ON r.survey_id = s.id
		 JOIN evenings e ON s.evening_id = e.id
		 WHERE e.group_id = ? AND e.date BETWEEN ? AND ?
		 ORDER BY e.date`, groupID, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err != nil {
		return nil, fmt.Errorf("get trend: %w", err)
	}
	defer rows.Close()

	type bucket struct {
		date     time.Time
		totalSum float64
		count    int
		qSums    map[string]float64
		qCounts  map[string]int
	}
	byDate := map[string]*bucket{}

	for rows.Next() {
		var date time.Time
		var aJSON string
		if err := rows.Scan(&date, &aJSON); err != nil {
			return nil, err
		}
		key := date.Format("2006-01-02")
		if byDate[key] == nil {
			byDate[key] = &bucket{
				date:    date,
				qSums:   make(map[string]float64),
				qCounts: make(map[string]int),
			}
		}

		var answers map[string]any
		if err := json.Unmarshal([]byte(aJSON), &answers); err != nil {
			continue
		}

		d := byDate[key]
		d.count++

		var ratingSum float64
		var ratingCount int
		for _, q := range ratingQs {
			v := toFloat(answers[q.ID])
			if v <= 0 {
				continue
			}
			ratingSum += v
			ratingCount++
			d.qSums[q.ID] += v
			d.qCounts[q.ID]++
		}
		if ratingCount > 0 {
			d.totalSum += ratingSum / float64(ratingCount)
		}
	}

	trend := &GroupTrend{}
	for _, d := range byDate {
		if d.count == 0 {
			continue
		}
		ratings := make([]RatingAvg, 0, len(ratingQs))
		for _, q := range ratingQs {
			avg := 0.0
			if d.qCounts[q.ID] > 0 {
				avg = round2(d.qSums[q.ID] / float64(d.qCounts[q.ID]))
			}
			ratings = append(ratings, RatingAvg{
				QuestionID:   q.ID,
				QuestionText: q.Text,
				Avg:          avg,
			})
		}
		trend.Points = append(trend.Points, TrendPoint{
			Date:       d.date,
			AvgOverall: round2(d.totalSum / float64(d.count)),
			Ratings:    ratings,
			Responses:  d.count,
		})
	}
	sort.Slice(trend.Points, func(i, j int) bool {
		return trend.Points[i].Date.Before(trend.Points[j].Date)
	})
	return trend, rows.Err()
}

func (s *Store) GetGroupComparisons(ctx context.Context) ([]GroupComparison, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.id, g.name,
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
		if err := rows.Scan(&c.GroupID, &c.GroupName, &c.TotalDAs, &c.TotalResp); err != nil {
			return nil, err
		}
		comps = append(comps, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Durchschnitt über alle Bewertungsfragen pro Gruppe berechnen
	for i, c := range comps {
		avg, ratings, err := s.calcGroupAvg(ctx, c.GroupID)
		if err == nil {
			comps[i].AvgOverall = avg
			comps[i].Ratings = ratings
		}
	}

	return comps, nil
}

func (s *Store) calcGroupAvg(ctx context.Context, groupID int) (float64, []RatingAvg, error) {
	ratingQs := ratingQuestions(survey.StandardQuestions)

	rows, err := s.db.QueryContext(ctx,
		`SELECT r.answers FROM responses r
		 JOIN surveys s ON r.survey_id = s.id
		 JOIN evenings e ON s.evening_id = e.id
		 WHERE e.group_id = ?`, groupID)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	var totalAvg float64
	var count int
	qSums := make(map[string]float64)
	qCounts := make(map[string]int)

	for rows.Next() {
		var aJSON string
		if err := rows.Scan(&aJSON); err != nil {
			continue
		}
		var answers map[string]any
		if err := json.Unmarshal([]byte(aJSON), &answers); err != nil {
			continue
		}

		var rSum float64
		var rCount int
		for _, q := range ratingQs {
			v := toFloat(answers[q.ID])
			if v <= 0 {
				continue
			}
			rSum += v
			rCount++
			qSums[q.ID] += v
			qCounts[q.ID]++
		}
		if rCount > 0 {
			totalAvg += rSum / float64(rCount)
			count++
		}
	}

	ratings := make([]RatingAvg, 0, len(ratingQs))
	for _, q := range ratingQs {
		avg := 0.0
		if qCounts[q.ID] > 0 {
			avg = round2(qSums[q.ID] / float64(qCounts[q.ID]))
		}
		ratings = append(ratings, RatingAvg{
			QuestionID:   q.ID,
			QuestionText: q.Text,
			Avg:          avg,
		})
	}

	if count == 0 {
		return 0, ratings, nil
	}
	return round2(totalAvg / float64(count)), ratings, nil
}

func ratingQuestions(questions []survey.Question) []survey.Question {
	var rqs []survey.Question
	for _, q := range questions {
		if q.Type == survey.TypeSchulnote || q.Type == survey.TypeStars {
			rqs = append(rqs, q)
		}
	}
	return rqs
}

type jsonScanner struct {
	target any
}

func scanJSON(target any) *jsonScanner {
	return &jsonScanner{target: target}
}

func (js *jsonScanner) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return fmt.Errorf("unsupported type for JSON scan: %T", src)
	}
	return json.Unmarshal(data, js.target)
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
