package workerpool

import (
	"HAB/internal/logger"
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type WorkerPool struct {
	workers      []*Worker
	numOfWorkers int
	rdb          *redis.Client
	stream       string
	group        string
}

func NewWorkerPool(numOfWorkers int, rdb *redis.Client, stream, group string) *WorkerPool {

	return &WorkerPool{
		workers:      make([]*Worker, numOfWorkers),
		numOfWorkers: numOfWorkers,
		rdb:          rdb,
		stream:       stream,
		group:        group,
	}
}

func (p *WorkerPool) Start(ctx context.Context) error {

	_, err := p.rdb.XGroupCreateMkStream(ctx, p.stream, p.group, "$").Result()

	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		return fmt.Errorf("failed to create consumer group: %v", err)
	}

	for i := 0; i < p.numOfWorkers; i++ {

		worker := NewWorker(
			fmt.Sprintf("Worker ID: %d", i+1),
			p.rdb,
			p.stream,
			p.group,
		)

		worker.Start(ctx)

		p.workers[i] = worker

		logger.Log.Info("Starting worker",
			zap.String("worker: ", worker.id))

	}

	logger.Log.Info("Successfully starting new worker group",
		zap.Int("Number of workers:  ", p.numOfWorkers))

	return nil
}

func (p *WorkerPool) Stop() {
	for _, worker := range p.workers {
		worker.Stop()
	}
}
