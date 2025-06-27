package models

type ProblemListItem struct {
	ID         int    `db:"id" json:"id"`
	Title      string `db:"title" json:"title"`
	Difficulty string `db:"difficulty" json:"difficulty"`
	IsSolved   bool   `json:"is_solved"`
}

type ProblemDetail struct {
	ID                  int            `db:"id" json:"id"`
	Title               string         `db:"title" json:"title"`
	Description         string         `db:"description" json:"description"`
	Difficulty          string         `db:"difficulty" json:"difficulty"`
	SampleInput         string         `db:"sample_input" json:"sample_input"`
	SampleOutput        string         `db:"sample_output" json:"sample_output"`
	StarterCode         map[int]string `json:"starter_code,omitempty"`
	IsSolved            bool           `json:"is_solved"`
	TotalSubmissions    int            `json:"total_submissions"`
	AcceptedSubmissions int            `json:"accepted_submissions"`
	AcceptanceRate      float64        `json:"acceptance_rate"`
}
