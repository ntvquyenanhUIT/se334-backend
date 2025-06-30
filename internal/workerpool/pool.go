package workerpool

import (
	"HAB/internal/logger"
	"HAB/internal/models"
	"HAB/internal/repositories"
	"HAB/internal/services"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// CodeWorker is a specialized worker that processes code submissions
type CodeWorker struct {
	id         string
	quit       chan bool
	rdb        *redis.Client
	stream     string
	group      string
	codeRepo   repositories.CodeRepository
	codeRunner *services.CodeRunnerService
}

// NewCodeWorker creates a new code worker
func NewCodeWorker(id string, rdb *redis.Client, stream, group string,
	codeRepo repositories.CodeRepository, codeRunner *services.CodeRunnerService) *CodeWorker {
	return &CodeWorker{
		id:         id,
		quit:       make(chan bool),
		rdb:        rdb,
		stream:     stream,
		group:      group,
		codeRepo:   codeRepo,
		codeRunner: codeRunner,
	}
}

// Start begins processing jobs from the stream
func (w *CodeWorker) Start(ctx context.Context) {
	go func() {
		for {
			select {
			case <-w.quit:
				return
			default:
				entries, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group:    w.group,
					Consumer: w.id,
					Streams:  []string{w.stream, ">"},
					Count:    1,
					Block:    5 * time.Second,
				}).Result()

				if err != nil {
					if err != redis.Nil {
						logger.Log.Error("Redis operation failed",
							zap.String("worker_id", w.id),
							zap.Error(err))
					}
					continue
				}

				for _, stream := range entries {
					for _, msg := range stream.Messages {
						w.processCodeJob(ctx, msg)
					}
				}
			}
		}
	}()
}

func (w *CodeWorker) Stop() {
	logger.Log.Info("Closing worker",
		zap.String("worker_id", w.id))
	w.quit <- true
	close(w.quit)
}

func (w *CodeWorker) processCodeJob(ctx context.Context, msg redis.XMessage) {
	logger.Log.Info("Processing code submission job",
		zap.String("worker_id", w.id),
		zap.String("job_id", msg.ID))

	if err := w.rdb.XAck(ctx, w.stream, w.group, msg.ID).Err(); err != nil {
		logger.Log.Error("Failed to acknowledge job",
			zap.String("worker_id", w.id),
			zap.Error(err))
	}

	submissionIDStr, ok := msg.Values["submission_id"].(string)
	if !ok {
		logger.Log.Error("Invalid submission ID in message",
			zap.String("worker_id", w.id),
			zap.Any("values", msg.Values))
		return
	}

	submissionID, err := strconv.Atoi(submissionIDStr)
	if err != nil {
		logger.Log.Error("Failed to parse submission ID",
			zap.String("worker_id", w.id),
			zap.String("submission_id", submissionIDStr),
			zap.Error(err))
		return
	}

	submission, err := w.codeRepo.GetSubmission(ctx, submissionID)
	if err != nil {
		logger.Log.Error("Failed to get submission",
			zap.String("worker_id", w.id),
			zap.Int("submission_id", submissionID),
			zap.Error(err))
		return
	}

	languageName, _, err := services.GetLanguageConfig(submission.LanguageID)
	if err != nil {
		logger.Log.Error("Failed to get language config",
			zap.String("worker_id", w.id),
			zap.Int("submission_id", submissionID),
			zap.Int("language_id", submission.LanguageID),
			zap.Error(err))

		// Update submission with error
		errorMsg := fmt.Sprintf("Unsupported language ID: %d", submission.LanguageID)
		err = w.codeRepo.UpdateSubmissionStatus(ctx, submissionID, models.StatusCompilationError, nil, &errorMsg)
		if err != nil {
			logger.Log.Error("Failed to update submission status", zap.Error(err))
		}
		return
	}

	testCases, err := w.codeRepo.GetTestCases(ctx, submission.ProblemID)
	if err != nil {
		logger.Log.Error("Failed to get test cases",
			zap.String("worker_id", w.id),
			zap.Int("submission_id", submissionID),
			zap.Int("problem_id", submission.ProblemID),
			zap.Error(err))

		errorMsg := "Failed to retrieve test cases"
		err = w.codeRepo.UpdateSubmissionStatus(ctx, submissionID, models.StatusCompilationError, nil, &errorMsg)
		if err != nil {
			logger.Log.Error("Failed to update submission status", zap.Error(err))
		}
		return
	}

	systemCode, err := w.codeRepo.GetSystemCode(ctx, submission.ProblemID, submission.LanguageID)
	if err != nil {
		logger.Log.Error("Failed to get system code",
			zap.String("worker_id", w.id),
			zap.Int("submission_id", submissionID),
			zap.Int("problem_id", submission.ProblemID),
			zap.Int("language_id", submission.LanguageID),
			zap.Error(err))

		errorMsg := "Failed to retrieve system code"
		err = w.codeRepo.UpdateSubmissionStatus(ctx, submissionID, models.StatusCompilationError, nil, &errorMsg)
		if err != nil {
			logger.Log.Error("Failed to update submission status", zap.Error(err))
		}
		return
	}

	importCode, err := w.codeRepo.GetLanguageImports(ctx, submission.ProblemID, submission.LanguageID)
	if err != nil {
		logger.Log.Error("Failed to get language imports",
			zap.String("worker_id", w.id),
			zap.Int("submission_id", submissionID),
			zap.Int("problem_id", submission.ProblemID),
			zap.Int("language_id", submission.LanguageID),
			zap.Error(err))

		errorMsg := "Failed to retrieve language imports"
		err = w.codeRepo.UpdateSubmissionStatus(ctx, submissionID, models.StatusCompilationError, nil, &errorMsg)
		if err != nil {
			logger.Log.Error("Failed to update submission status", zap.Error(err))
		}
		return
	}

	request := services.CodeRunnerRequest{
		Submission:   *submission,
		TestCases:    testCases,
		SystemCode:   systemCode,
		ImportCode:   importCode,
		LanguageName: languageName,
	}

	// Execute code
	result, err := w.codeRunner.Execute(ctx, request)
	if err != nil {
		logger.Log.Error("Code execution failed",
			zap.String("worker_id", w.id),
			zap.Int("submission_id", submissionID),
			zap.Error(err))

		// Update submission with error
		errorMsg := fmt.Sprintf("Execution error: %v", err)
		err = w.codeRepo.UpdateSubmissionStatus(ctx, submissionID, models.StatusCompilationError, nil, &errorMsg)
		if err != nil {
			logger.Log.Error("Failed to update submission status", zap.Error(err))
		}
		return
	}

	err = w.codeRepo.UpdateSubmissionStatus(ctx, submissionID, result.Status, result.FailedTestID, result.FailedOutput)
	if err != nil {
		logger.Log.Error("Failed to update submission status",
			zap.String("worker_id", w.id),
			zap.Int("submission_id", submissionID),
			zap.Error(err))
		return
	}

	logger.Log.Info("Finished processing code submission job",
		zap.String("worker_id", w.id),
		zap.String("job_id", msg.ID),
		zap.String("status", result.Status),
		zap.Duration("execution_time", result.ExecutionTime))
}

type CodeWorkerPool struct {
	workers    []*CodeWorker
	numWorkers int
	rdb        *redis.Client
	stream     string
	group      string
	codeRepo   repositories.CodeRepository
	codeRunner *services.CodeRunnerService
}

func NewCodeWorkerPool(numWorkers int, rdb *redis.Client, stream, group string,
	codeRepo repositories.CodeRepository) (*CodeWorkerPool, error) {
	codeRunner, err := services.NewCodeRunnerService("/tmp/code-execution")
	if err != nil {
		return nil, fmt.Errorf("failed to create code runner service: %w", err)
	}

	return &CodeWorkerPool{
		workers:    make([]*CodeWorker, numWorkers),
		numWorkers: numWorkers,
		rdb:        rdb,
		stream:     stream,
		group:      group,
		codeRepo:   codeRepo,
		codeRunner: codeRunner,
	}, nil
}

func (p *CodeWorkerPool) Start(ctx context.Context) error {
	// Create consumer group if it doesn't exist
	_, err := p.rdb.XGroupCreateMkStream(ctx, p.stream, p.group, "$").Result()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Start workers
	for i := 0; i < p.numWorkers; i++ {
		worker := NewCodeWorker(
			fmt.Sprintf("CodeWorker-%d", i+1),
			p.rdb,
			p.stream,
			p.group,
			p.codeRepo,
			p.codeRunner,
		)

		worker.Start(ctx)
		p.workers[i] = worker

		logger.Log.Info("Starting code worker",
			zap.String("worker_id", worker.id))
	}

	logger.Log.Info("Code worker pool started",
		zap.Int("num_workers", p.numWorkers))

	return nil
}

// Stop terminates all workers in the pool
func (p *CodeWorkerPool) Stop() {
	for _, worker := range p.workers {
		worker.Stop()
	}
}
