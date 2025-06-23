package configs

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DBHost          string
	DBPort          string
	DBUser          string
	DBPassword      string
	DBName          string
	ServerPort      string
	NumberOfWorkers int
}

func LoadConfig() *Config {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file", err)
	}

	numWorkerInt, _ := strconv.Atoi(os.Getenv("NUM_OF_WORKERS"))

	return &Config{
		DBHost:          os.Getenv("DB_HOST"),
		DBPort:          os.Getenv("DB_PORT"),
		DBUser:          os.Getenv("DB_USER"),
		DBPassword:      os.Getenv("DB_PASSWORD"),
		DBName:          os.Getenv("DB_NAME"),
		ServerPort:      os.Getenv("SEVER_PORT"),
		NumberOfWorkers: numWorkerInt,
	}
}
