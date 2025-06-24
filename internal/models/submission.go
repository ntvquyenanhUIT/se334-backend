package models

import (
	"errors"
	"strings"
	"time"
)

const (
	StatusAccepted         = "ACCEPTED"
	StatusWrongAnswer      = "WRONG_ANSWER"
	StatusCompilationError = "COMPILATION_ERROR"
	StatusPending          = "PENDING"
	StatusProcessing       = "PROCESSING"
)

type Submission struct {
	ID            int       `db:"id" json:"id"`
	UserID        int       `db:"user_id" json:"user_id"`
	ProblemID     int       `db:"problem_id" json:"problem_id"`
	LanguageID    int       `db:"language_id" json:"language_id"`
	SourceCode    string    `db:"source_code" json:"source_code"`
	Status        string    `db:"status" json:"status"`
	WrongTestcase *int      `db:"wrong_testcase" json:"wrong_testcase,omitempty"`
	ProgramOutput *string   `db:"program_output" json:"program_output,omitempty"`
	SubmittedAt   time.Time `db:"submitted_at" json:"submitted_at"`
}

type SubmissionResponse struct {
	Status         string  `json:"status"`
	WrongTestcase  *string `json:"wrong_testcase,omitempty"`
	ExpectedOutput *string `json:"expected_output,omitempty"`
	ProgramOutput  *string `json:"program_output,omitempty"`
	SourceCode     string  `json:"source_code"`
}

type SubmissionRequest struct {
	ProblemID  int    `json:"problem_id" binding:"required"`
	LanguageID int    `json:"language_id" binding:"required"`
	SourceCode string `json:"source_code" binding:"required"`
}

type SubmissionListItem struct {
	ID          int       `db:"id" json:"id"`
	LanguageID  int       `db:"language_id" json:"language_id"`
	Status      string    `db:"status" json:"status"`
	SubmittedAt time.Time `db:"submitted_at" json:"submitted_at"`
	// Derived field filled in by the handler
	FormattedTime string `db:"-" json:"submitted_time"`
	LanguageName  string `db:"-" json:"language_name"`
}

func (r *SubmissionRequest) ValidateRequest() error {

	if r.ProblemID <= 0 {
		return errors.New("problem ID must be a positive integer")
	}

	if r.LanguageID <= 0 {
		return errors.New("language ID must be a positive integer")
	}

	if strings.TrimSpace(r.SourceCode) == "" {
		return errors.New("source code cannot be empty")
	}

	return nil
}
