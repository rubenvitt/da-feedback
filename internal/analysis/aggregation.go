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

	err := s.db.QueryRowContext(ctx,
		"SELECT date, topic, participant_count FROM evenings WHERE id = ?", eveningID,
	).Scan(&stats.Date, &stats.Topic, &stats.ParticipantCount)
	if err != nil {
		return nil, fmt.Errorf("get evening: %w", err)
	}

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

	byDate := map[string]*struct {
		date                                 time.Time
		sumOverall, sumRelevance, sumClarity float64
		count                                int
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
				date                                 time.Time
				sumOverall, sumRelevance, sumClarity float64
				count                                int
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
