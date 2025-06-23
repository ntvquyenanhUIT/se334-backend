package repositories

import (
	"HAB/internal/models"
	"HAB/internal/services"
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type CodeRepository interface {
	GetSubmission(ctx context.Context, submissionID int) (*models.Submission, error)
	GetSubmissionByID(ctx context.Context, submissionID int) (*models.SubmissionResponse, error)
	GetTestCases(ctx context.Context, problemID int) ([]services.TestCase, error)
	GetSystemCode(ctx context.Context, problemID int, languageID int) (string, error)
	GetLanguageImports(ctx context.Context, problemID int, languageID int) (string, error)
	CreateSubmission(ctx context.Context, submission *models.Submission) error
	UpdateSubmissionStatus(ctx context.Context, submissionID int, status string, wrongTestcase *int, wrongOutput *string) error
}

type codeRepository struct {
	db *sqlx.DB
}

func NewCodeRepository(db *sqlx.DB) CodeRepository {
	return &codeRepository{db: db}
}

func (r *codeRepository) GetSubmission(ctx context.Context, submissionID int) (*models.Submission, error) {
	query := `SELECT id, user_id, problem_id, language_id, source_code, status, 
                  wrong_testcase, program_output, submitted_at 
              FROM submissions WHERE id = ?`

	var submission models.Submission

	err := r.db.GetContext(ctx, &submission, query, submissionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("submission not found: %d", submissionID)
		}
		return nil, fmt.Errorf("failed to get submission: %w", err)
	}

	return &submission, nil
}
func (r *codeRepository) GetSubmissionByID(ctx context.Context, submissionID int) (*models.SubmissionResponse, error) {
	query := `SELECT id, user_id, problem_id, language_id, source_code, status, 
              wrong_testcase, program_output, submitted_at 
              FROM submissions WHERE id = ?`

	var submission models.Submission

	err := r.db.GetContext(ctx, &submission, query, submissionID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("submission not found: %d", submissionID)
		}
		return nil, fmt.Errorf("failed to get submission: %w", err)
	}

	response := &models.SubmissionResponse{
		Status:        submission.Status,
		ProgramOutput: submission.ProgramOutput,
		SourceCode:    submission.SourceCode,
	}
	fmt.Printf("Source code ne: %v\n", response.SourceCode)
	if submission.WrongTestcase != nil {
		testcaseQuery := `SELECT input, expected_output FROM test_cases WHERE id = ?`

		var testcase struct {
			Input          string `db:"input"`
			ExpectedOutput string `db:"expected_output"`
		}

		err := r.db.GetContext(ctx, &testcase, testcaseQuery, *submission.WrongTestcase)
		if err != nil {
			if err != sql.ErrNoRows {
				return nil, fmt.Errorf("failed to get testcase data: %w", err)
			}
		} else {
			response.WrongTestcase = &testcase.Input
			response.ExpectedOutput = &testcase.ExpectedOutput
		}
	}

	return response, nil
}

func (r *codeRepository) GetTestCases(ctx context.Context, problemID int) ([]services.TestCase, error) {
	query := `SELECT id, input, expected_output FROM test_cases WHERE problem_id = ?`

	var testCases []struct {
		ID       int    `db:"id"`
		Input    string `db:"input"`
		Expected string `db:"expected_output"`
	}

	if err := r.db.SelectContext(ctx, &testCases, query, problemID); err != nil {
		return nil, fmt.Errorf("failed to get test cases: %w", err)
	}

	result := make([]services.TestCase, len(testCases))
	for i, tc := range testCases {
		result[i] = services.TestCase{
			ID:       tc.ID,
			Input:    tc.Input,
			Expected: tc.Expected,
		}
	}

	return result, nil
}

func (r *codeRepository) GetSystemCode(ctx context.Context, problemID int, languageID int) (string, error) {
	query := `SELECT code FROM system_code WHERE problem_id = ? AND language_id = ?`

	var code string
	if err := r.db.GetContext(ctx, &code, query, problemID, languageID); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("system code not found for problem %d and language %d", problemID, languageID)
		}
		return "", fmt.Errorf("failed to get system code: %w", err)
	}

	return code, nil
}

func (r *codeRepository) GetLanguageImports(ctx context.Context, problemID int, languageID int) (string, error) {
	query := `SELECT code FROM language_imports WHERE problem_id = ? AND language_id = ?`

	var code string
	if err := r.db.GetContext(ctx, &code, query, problemID, languageID); err != nil {
		if err == sql.ErrNoRows {
			return "", fmt.Errorf("language imports not found for problem %d and language %d", problemID, languageID)
		}
		return "", fmt.Errorf("failed to get language imports: %w", err)
	}

	return code, nil
}

func (r *codeRepository) CreateSubmission(ctx context.Context, submission *models.Submission) error {
	query := `INSERT INTO submissions (user_id, problem_id, language_id, source_code, status) 
              VALUES (?, ?, ?, ?, ?)`

	result, err := r.db.ExecContext(ctx, query,
		submission.UserID,
		submission.ProblemID,
		submission.LanguageID,
		submission.SourceCode,
		submission.Status,
	)

	if err != nil {
		return fmt.Errorf("failed to create submission: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert ID: %w", err)
	}

	submission.ID = int(id)
	return nil
}

func (r *codeRepository) UpdateSubmissionStatus(ctx context.Context, submissionID int, status string, wrongTestcase *int, wrongOutput *string) error {
	query := `UPDATE submissions SET status = ?, wrong_testcase = ?, program_output = ? WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, status, wrongTestcase, wrongOutput, submissionID)
	if err != nil {
		return fmt.Errorf("failed to update submission status: %w", err)
	}

	return nil
}
