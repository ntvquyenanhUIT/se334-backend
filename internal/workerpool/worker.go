package workerpool

import (
	"context"
	"time"

	"HAB/internal/logger"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Worker struct {
	id     string
	quit   chan bool
	rdb    *redis.Client
	stream string
	group  string
}

func NewWorker(id string, rdb *redis.Client, stream, group string) *Worker {

	return &Worker{
		id:     id,
		quit:   make(chan bool),
		rdb:    rdb,
		stream: stream,
		group:  group,
	}

}

func (w *Worker) Start(ctx context.Context) {
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
						w.processJob(ctx, msg)
					}
				}
			}
		}
	}()
}

func (w *Worker) processJob(ctx context.Context, msg redis.XMessage) {
	logger.Log.Info("Processing Job",
		zap.String("worker_id", w.id),
		zap.String("job_id", msg.ID))

	if err := w.rdb.XAck(ctx, w.stream, w.group, msg.ID).Err(); err != nil {
		logger.Log.Info("Failed to acknowledge job",
			zap.String("worker_id", w.id),
			zap.Error(err))
	}

	logger.Log.Info("Finished Processing Job",
		zap.String("worker_id", w.id),
		zap.String("job_id", msg.ID))

}

func (w *Worker) Stop() {
	logger.Log.Info("Closing worker",
		zap.String("worker_id", w.id))
	w.quit <- true
	close(w.quit)
}
