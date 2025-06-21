package dbs

import (
	config "HAB/configs"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
)

var DB *sqlx.DB

func Init() (*sqlx.DB, error) {
	cfg := config.LoadConfig()

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword,
		cfg.DBHost, cfg.DBPort,
		cfg.DBName,
	)

	var err error
	DB, err = sqlx.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	return DB, nil
}
