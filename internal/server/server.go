package server

import (
	"HAB/configs"
	"HAB/internal/dbs"
	"HAB/internal/handlers"
	"HAB/internal/logger"
	"HAB/internal/middlewares"
	"HAB/internal/repositories"
	"HAB/internal/services"
	"HAB/internal/workerpool"

	"context"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
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

	codeRepo := repositories.NewCodeRepository(db)
	problemRepo := repositories.NewProblemRepository(db)
	userRepo := repositories.NewUserRepository(db)

	tokenService := services.NewTokenService(config.JWTSecret)

	workerPool, err := workerpool.NewCodeWorkerPool(config.NumberOfWorkers, dbs.RedisClient, "code_submissions", "judgers", codeRepo)
	if err != nil {
		logger.Log.Error("Failed initializing worker pool")
		log.Fatalf("failed to initialize worker pool: %v", err)
	}

	if err := workerPool.Start(ctx); err != nil {
		logger.Log.Error("Failed starting worker pool")
		log.Fatalf("failed to start worker pool: %v", err)
	}
	defer workerPool.Stop()

	submissionHandler := handlers.NewSubmissionHandler(codeRepo, dbs.RedisClient)
	problemHandler := handlers.NewProblemHandler(problemRepo)
	authHandler := handlers.NewAuthHandler(userRepo, tokenService)

	router := gin.New()
	router.Use(middlewares.ErrorHandlerMiddleware())
	router.Use(cors.Default())

	authMiddleware := middlewares.AuthMiddleware(tokenService)
	optionalAuthMiddleware := middlewares.OptionalAuthMiddleware(tokenService)

	submissionHandler.RegisterRoutes(router, authMiddleware)
	problemHandler.RegisterRoutes(router, optionalAuthMiddleware)
	authHandler.RegisterRoutes(router)

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	port := ":" + config.ServerPort
	log.Printf("Starting server on port %s", port)
	if err := router.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
