package db

import (
	"log"

	"github.com/roypratim/skyhigh/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a PostgreSQL connection and returns a *gorm.DB instance.
func Connect(cfg *config.Config) *gorm.DB {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)

	log.Println("PostgreSQL connected")
	return db
}
