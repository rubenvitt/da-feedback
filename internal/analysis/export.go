package analysis

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

func (s *Store) ExportGroupCSV(ctx context.Context, w io.Writer, groupID int, from, to time.Time) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.date, e.topic, e.participant_count
		 FROM evenings e WHERE e.group_id = ? AND e.date BETWEEN ? AND ?
		 ORDER BY e.date`, groupID, from, to)
	if err != nil {
		return fmt.Errorf("query evenings: %w", err)
	}
	defer rows.Close()

	headerWritten := false

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

		if !headerWritten {
			header := []string{"Datum", "Thema", "Teilnehmer", "Antworten", "Gesamt (Ø)"}
			for _, r := range stats.Ratings {
				header = append(header, r.QuestionText+" (Ø)")
			}
			for _, t := range stats.TextAnswers {
				header = append(header, t.QuestionText)
			}
			if err := cw.Write(header); err != nil {
				return err
			}
			headerWritten = true
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
		}
		for _, r := range stats.Ratings {
			record = append(record, fmt.Sprintf("%.2f", r.Avg))
		}
		for _, t := range stats.TextAnswers {
			record = append(record, joinStrings(t.Responses))
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}

	if !headerWritten {
		if err := cw.Write([]string{"Keine Daten"}); err != nil {
			return err
		}
	}

	return rows.Err()
}

func joinStrings(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	var escaped []string
	for _, s := range ss {
		escaped = append(escaped, strings.ReplaceAll(s, ";", ","))
	}
	b, _ := json.Marshal(escaped)
	return string(b)
}
