package analysis

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

func (s *Store) ExportGroupCSV(ctx context.Context, w io.Writer, groupID int, from, to time.Time) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{"Datum", "Thema", "Teilnehmer", "Antworten",
		"Gesamt (Ø)", "Praxisrelevanz (Ø)", "Verständlichkeit (Ø)",
		"Highlights", "Verbesserungen", "Themenwünsche"}
	if err := cw.Write(header); err != nil {
		return err
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT e.id, e.date, e.topic, e.participant_count
		 FROM evenings e WHERE e.group_id = ? AND e.date BETWEEN ? AND ?
		 ORDER BY e.date`, groupID, from, to)
	if err != nil {
		return fmt.Errorf("query evenings: %w", err)
	}
	defer rows.Close()

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
			fmt.Sprintf("%.2f", stats.AvgRelevance),
			fmt.Sprintf("%.2f", stats.AvgClarity),
			joinStrings(stats.Highlights),
			joinStrings(stats.Improvements),
			joinStrings(stats.TopicWishes),
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	return rows.Err()
}

func joinStrings(ss []string) string {
	b, _ := json.Marshal(ss)
	return string(b)
}
