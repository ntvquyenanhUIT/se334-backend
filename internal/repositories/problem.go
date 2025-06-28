package repositories

import (
	"HAB/internal/logger"
	"HAB/internal/models"
	"HAB/internal/services"
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

type ProblemRepository interface {
	GetProblems(ctx context.Context) ([]models.ProblemListItem, error)
	GetProblemByID(ctx context.Context, problemID int) (*models.ProblemDetail, error)
	GetStarterCode(ctx context.Context, problemID int) (map[int]string, error)
	GetSolvedProblemIDs(ctx context.Context, userID int) (map[int]bool, error)
}

type problemRepository struct {
	db    *sqlx.DB
	cache services.Cache
}

func NewProblemRepository(db *sqlx.DB, cache services.Cache) ProblemRepository {
	return &problemRepository{db: db, cache: cache}
}

func (r *problemRepository) GetProblems(ctx context.Context) ([]models.ProblemListItem, error) {
	cacheKey := "problems:list"
	var problems []models.ProblemListItem

	if err := r.cache.Get(ctx, cacheKey, &problems); err == nil {
		logger.Log.Info("Cache hit, returning problem list...")
		return problems, nil // Cache hit
	}

	logger.Log.Info("Problem list not in cache, retrieving database")

	query := `SELECT id, title, difficulty FROM problems`
	if err := r.db.SelectContext(ctx, &problems, query); err != nil {
		return nil, fmt.Errorf("failed to get problems: %w", err)
	}

	_ = r.cache.Set(ctx, cacheKey, problems, 4*time.Hour)

	return problems, nil
}

func (r *problemRepository) GetProblemByID(ctx context.Context, problemID int) (*models.ProblemDetail, error) {
	cacheKey := fmt.Sprintf("problem:%d", problemID)
	var problem models.ProblemDetail

	if err := r.cache.Get(ctx, cacheKey, &problem); err == nil {
		logger.Log.Info("Cache hit, returning problem details...")
		return &problem, nil
	}
	logger.Log.Info("Problem details not in cache, retrieving database")
	query := `SELECT id, title, description, difficulty, sample_input, sample_output 
              FROM problems WHERE id = ?`

	if err := r.db.GetContext(ctx, &problem, query, problemID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("problem not found: %d", problemID)
		}
		return nil, fmt.Errorf("failed to get problem: %w", err)
	}

	statsQuery := `
        SELECT 
            COUNT(*) as total_submissions,
            COUNT(CASE WHEN status = 'ACCEPTED' THEN 1 END) as accepted_submissions
        FROM submissions 
        WHERE problem_id = ?`

	var stats struct {
		TotalSubmissions    int `db:"total_submissions"`
		AcceptedSubmissions int `db:"accepted_submissions"`
	}
	if err := r.db.GetContext(ctx, &stats, statsQuery, problemID); err != nil {
		return nil, fmt.Errorf("failed to get submission stats: %w", err)
	}

	problem.TotalSubmissions = stats.TotalSubmissions
	problem.AcceptedSubmissions = stats.AcceptedSubmissions
	if stats.TotalSubmissions > 0 {
		problem.AcceptanceRate = (float64(stats.AcceptedSubmissions) / float64(stats.TotalSubmissions)) * 100
	} else {
		problem.AcceptanceRate = 0
	}

	starterCode, err := r.GetStarterCode(ctx, problemID)
	if err != nil {
		return nil, err
	}

	_ = r.cache.Set(ctx, cacheKey, problem, 4*time.Hour)

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
