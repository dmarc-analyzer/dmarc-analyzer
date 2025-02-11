package db

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func init() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set in environment")
	}
	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	fmt.Printf("%+v\n", err)
}
