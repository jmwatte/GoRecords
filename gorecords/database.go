package main

import (
	"gorecords/models"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// InitDB initializes the SQLite database using a pure Go driver (modernc.org/sqlite via glebarez/sqlite)
// to avoid requiring CGO. The database file is stored in the user's config directory.
func InitDB() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		slog.Error("failed to get user config directory", "error", err)
		configDir = "."
	}

	dbDir := filepath.Join(configDir, "gorecords")
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		slog.Error("failed to create database directory", "path", dbDir, "error", err)
	}

	dbPath := filepath.Join(dbDir, "gorecords.db")
	slog.Info("initializing database", "path", dbPath)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		slog.Error("failed to open database", "error", err)
		panic("failed to connect to database: " + err.Error())
	}

	// Enable WAL mode for better concurrent read performance
	db.Exec("PRAGMA journal_mode=WAL")
	// Enable foreign keys
	db.Exec("PRAGMA foreign_keys=ON")

	DB = db

	if err := db.AutoMigrate(&models.Track{}); err != nil {
		slog.Error("failed to auto-migrate database", "error", err)
		panic("failed to auto-migrate: " + err.Error())
	}

	slog.Info("database initialized successfully")
}

// GetDB returns the database instance.
func GetDB() *gorm.DB {
	return DB
}
