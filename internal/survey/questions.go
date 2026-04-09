package survey

var StandardQuestions = []Question{
	// Schulnoten (1 = Sehr gut, 6 = Ungenügend)
	{ID: "q1", Type: TypeSchulnote, Text: "Wie war der Dienstabend insgesamt?", Required: true, Standard: true},
	{ID: "q2", Type: TypeSchulnote, Text: "Wie spannend war das Thema für dich?", Required: true, Standard: true},
	{ID: "q3", Type: TypeSchulnote, Text: "Wurden deine Erwartungen erfüllt?", Required: true, Standard: true},
	{ID: "q4", Type: TypeSchulnote, Text: "Wie gut war der Abend strukturiert?", Required: true, Standard: true},
	{ID: "q5", Type: TypeSchulnote, Text: "Hat man den Aufwand dahinter gemerkt?", Required: true, Standard: true},
	{ID: "q6", Type: TypeSchulnote, Text: "Wie gut war alles vorbereitet?", Required: true, Standard: true},
	{ID: "q7", Type: TypeSchulnote, Text: "Wurdest du als Teilnehmer einbezogen?", Required: true, Standard: true},
	{ID: "q8", Type: TypeSchulnote, Text: "Hast du etwas Neues mitgenommen?", Required: true, Standard: true},

	// Freitext
	{ID: "q9", Type: TypeText, Text: "Was hat dir am besten gefallen?", Required: false, Standard: true},
	{ID: "q10", Type: TypeText, Text: "Worauf sollten wir beim nächsten Mal näher eingehen?", Required: false, Standard: true},
	{ID: "q11", Type: TypeText, Text: "Was könnten wir besser machen?", Required: false, Standard: true},
	{ID: "q12", Type: TypeText, Text: "Hast du einen Tipp für uns?", Required: false, Standard: true},
	{ID: "q13", Type: TypeText, Text: "Welches Thema würde dich als Nächstes interessieren?", Required: false, Standard: true},
	{ID: "q14", Type: TypeText, Text: "Gibt es sonst noch etwas, das du loswerden möchtest?", Required: false, Standard: true},
}

func NewQuestionsWithStandard(extra []Question) []Question {
	all := make([]Question, len(StandardQuestions))
	copy(all, StandardQuestions)
	return append(all, extra...)
}
