package survey

var StandardQuestions = []Question{
	{ID: "q1", Type: TypeStars, Text: "Wie bewertest du den heutigen Dienstabend insgesamt?", Required: true, Standard: true},
	{ID: "q2", Type: TypeStars, Text: "Wie praxisrelevant waren die Inhalte?", Required: true, Standard: true},
	{ID: "q3", Type: TypeStars, Text: "Wie verständlich war die Vermittlung?", Required: true, Standard: true},
	{ID: "q4", Type: TypeText, Text: "Was hat dir besonders gut gefallen?", Required: false, Standard: true},
	{ID: "q5", Type: TypeText, Text: "Was könnte beim nächsten Mal besser laufen?", Required: false, Standard: true},
	{ID: "q6", Type: TypeText, Text: "Welches Thema wünschst du dir für einen kommenden DA?", Required: false, Standard: true},
}

func NewQuestionsWithStandard(extra []Question) []Question {
	all := make([]Question, len(StandardQuestions))
	copy(all, StandardQuestions)
	return append(all, extra...)
}
