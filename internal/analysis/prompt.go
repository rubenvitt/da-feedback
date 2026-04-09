package analysis

import (
	"fmt"
	"strings"

	"github.com/rubeen/da-feedback/internal/evening"
	"github.com/rubeen/da-feedback/internal/group"
	"github.com/rubeen/da-feedback/internal/survey"
)

func BuildPrompt(stats *DAStats, responses []survey.Response, grp group.Group, eve evening.Evening) string {
	var b strings.Builder

	b.WriteString("Du bist ein erfahrener Ausbildungsberater beim Deutschen Roten Kreuz. ")
	b.WriteString("Analysiere die folgenden anonymisierten Rückmeldungen zu einem Dienstabend ")
	b.WriteString("einer Rotkreuz-Gruppe sachlich und konstruktiv.\n\n")
	b.WriteString("Erstelle eine strukturierte Auswertung auf Deutsch mit folgenden Abschnitten:\n")
	b.WriteString("1. **Stärken** - Was lief besonders gut? Beziehe dich auf konkrete Bewertungen und Aussagen.\n")
	b.WriteString("2. **Verbesserungspotenzial** - Welche Bereiche schneiden schlechter ab oder werden kritisiert?\n")
	b.WriteString("3. **Konkrete Empfehlungen** - Was sollte beim nächsten Dienstabend konkret anders gemacht werden?\n")
	b.WriteString("4. **Fazit** - Ein kurzes zusammenfassendes Urteil.\n\n")
	b.WriteString("Halte den Bericht sachlich, wertschätzend und handlungsorientiert. ")
	b.WriteString("Beziehe dich auf konkrete Aussagen aus den Freitextantworten.\n\n")
	b.WriteString("---\n\n")

	// Metadaten
	b.WriteString("## Metadaten\n")
	fmt.Fprintf(&b, "- Gruppe: %s\n", grp.Name)
	fmt.Fprintf(&b, "- Datum: %s\n", eve.Date.Format("02.01.2006"))
	if eve.Topic != nil && *eve.Topic != "" {
		fmt.Fprintf(&b, "- Thema: %s\n", *eve.Topic)
	}
	fmt.Fprintf(&b, "- Anzahl Rückmeldungen: %d\n", stats.ResponseCount)
	if eve.ParticipantCount != nil {
		fmt.Fprintf(&b, "- Teilnehmer gesamt: %d\n", *eve.ParticipantCount)
	}
	b.WriteString("\n")

	// Durchschnittliche Bewertungen
	b.WriteString("## Durchschnittliche Bewertungen (Schulnoten: 1 = sehr gut, 6 = ungenügend)\n")
	for _, r := range stats.Ratings {
		fmt.Fprintf(&b, "- %s: %.2f\n", r.QuestionText, r.Avg)
	}
	fmt.Fprintf(&b, "- **Gesamtdurchschnitt: %.2f**\n\n", stats.AvgOverall)

	// Gesammelte Freitextantworten
	b.WriteString("## Freitextantworten (gesammelt)\n")
	for _, ta := range stats.TextAnswers {
		if len(ta.Responses) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s\n", ta.QuestionText)
		for _, resp := range ta.Responses {
			fmt.Fprintf(&b, "- %s\n", resp)
		}
	}
	b.WriteString("\n")

	// Einzelne Rückmeldungen
	b.WriteString("## Einzelne Rückmeldungen (Rohdaten)\n\n")

	ratingIDs := make([]string, 0, len(stats.Ratings))
	ratingText := make(map[string]string, len(stats.Ratings))
	for _, r := range stats.Ratings {
		ratingIDs = append(ratingIDs, r.QuestionID)
		ratingText[r.QuestionID] = r.QuestionText
	}
	textIDs := make([]string, 0, len(stats.TextAnswers))
	textLabel := make(map[string]string, len(stats.TextAnswers))
	for _, t := range stats.TextAnswers {
		textIDs = append(textIDs, t.QuestionID)
		textLabel[t.QuestionID] = t.QuestionText
	}

	for i, resp := range responses {
		fmt.Fprintf(&b, "### Rückmeldung %d\n", i+1)
		for _, qid := range ratingIDs {
			if v, ok := resp.Answers[qid]; ok {
				if f, isFloat := v.(float64); isFloat && f > 0 {
					fmt.Fprintf(&b, "- %s: %.0f\n", ratingText[qid], f)
				}
			}
		}
		for _, qid := range textIDs {
			if v, ok := resp.Answers[qid]; ok {
				if s, isStr := v.(string); isStr && strings.TrimSpace(s) != "" {
					fmt.Fprintf(&b, "- %s: %s\n", textLabel[qid], s)
				}
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
