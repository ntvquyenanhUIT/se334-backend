package repositories

import (
	"HAB/internal/models"
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type ProblemRepository interface {
	GetProblems(ctx context.Context) ([]models.ProblemListItem, error)
	GetProblemByID(ctx context.Context, problemID int) (*models.ProblemDetail, error)
	GetStarterCode(ctx context.Context, problemID int) (map[int]string, error)
	GetSolvedProblemIDs(ctx context.Context, userID int) (map[int]bool, error)
}

type problemRepository struct {
	db *sqlx.DB
}

func NewProblemRepository(db *sqlx.DB) ProblemRepository {
	return &problemRepository{db: db}
}

func (r *problemRepository) GetProblems(ctx context.Context) ([]models.ProblemListItem, error) {
	query := `SELECT id, title, difficulty FROM problems`

	var problems []models.ProblemListItem
	if err := r.db.SelectContext(ctx, &problems, query); err != nil {
		return nil, fmt.Errorf("failed to get problems: %w", err)
	}

	return problems, nil
}

func (r *problemRepository) GetProblemByID(ctx context.Context, problemID int) (*models.ProblemDetail, error) {
	query := `SELECT id, title, description, difficulty, sample_input, sample_output 
              FROM problems WHERE id = ?`

	var problem models.ProblemDetail
	if err := r.db.GetContext(ctx, &problem, query, problemID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("problem not found: %d", problemID)
		}
		return nil, fmt.Errorf("failed to get problem: %w", err)
	}

	// Get starter code for the problem
	starterCode, err := r.GetStarterCode(ctx, problemID)
	if err != nil {
		return nil, err
	}

	problem.StarterCode = starterCode
	return &problem, nil
}

func (r *problemRepository) GetStarterCode(ctx context.Context, problemID int) (map[int]string, error) {
	query := `SELECT language_id, code FROM starter_code WHERE problem_id = ?`

	type starterCodeRow struct {
		LanguageID int    `db:"language_id"`
		Code       string `db:"code"`
	}

	var rows []starterCodeRow
	if err := r.db.SelectContext(ctx, &rows, query, problemID); err != nil {
		return nil, fmt.Errorf("failed to get starter code: %w", err)
	}

	starterCode := make(map[int]string)
	for _, row := range rows {
		starterCode[row.LanguageID] = row.Code
	}

	return starterCode, nil
}

func (r *problemRepository) GetSolvedProblemIDs(ctx context.Context, userID int) (map[int]bool, error) {
	query := `SELECT DISTINCT problem_id FROM submissions WHERE user_id = ? AND status = 'ACCEPTED'`

	var problemIDs []int
	if err := r.db.SelectContext(ctx, &problemIDs, query, userID); err != nil {
		return nil, fmt.Errorf("failed to get solved problem IDs: %w", err)
	}

	solvedMap := make(map[int]bool, len(problemIDs))
	for _, id := range problemIDs {
		solvedMap[id] = true
	}

	return solvedMap, nil
}
