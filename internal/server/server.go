package server

import (
	"HAB/configs"
	"HAB/internal/dbs"
	"HAB/internal/logger"
	"HAB/internal/middlewares"
	"HAB/internal/workerpool"

	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func StartGinServer() {
	logger.InitLogger()
	defer logger.SyncLogger()

	config := configs.LoadConfig()

	db, err := dbs.Init()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	if err := dbs.InitRedis(ctx); err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer dbs.CloseRedis()

	workerpool := workerpool.NewWorkerPool(config.NumberOfWorkers, dbs.RedisClient, "code_submissions", "judgers")

	if err := workerpool.Start(ctx); err != nil {
		logger.Log.Error("Failed starting worker pool")
		log.Fatalf("failed to start worker pool: %v", err)
	}

	router := gin.New()
	router.Use(middlewares.ErrorHandlerMiddleware())

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	port := ":" + config.ServerPort
	log.Printf("Starting server on port %s", port)
	if err := router.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
