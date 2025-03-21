// Package db provides database connectivity and operations for the DMARC analyzer.
// It uses GORM as an ORM to interact with a PostgreSQL database.
package db

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DB is the global database connection instance used throughout the application.
// It's initialized during package initialization and provides access to the database.
var DB *gorm.DB

// init initializes the database connection when the package is imported.
// It reads the database connection string from the DATABASE_URL environment variable,
// establishes a connection to the PostgreSQL database using GORM,
// and sets up the global DB variable for use by other parts of the application.
func init() {
	// Get database connection string from environment variable
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set in environment")
	}
	
	// Initialize the database connection
	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	
	// Log any connection errors
	fmt.Printf("%+v\n", err)
}
