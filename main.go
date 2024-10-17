package main

import (
	"fmt"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	messageIDList := []string{}

	for _, messageID := range messageIDList {
		feedback, err := backend.ParseNewMail("", messageID)
		fmt.Printf("%+v %+v\n", feedback, err)

		reports := backend.ParseDmarcReport(feedback, messageID)
		fmt.Printf("%+v\n", reports)

		dsn := "host=localhost user=postgres password=postgres dbname=dmarc sslmode=disable"
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		fmt.Printf("%+v\n", err)
		result := db.Create(reports)
		if result.Error != nil {
			fmt.Printf("%+v\n", result.Error)
		}
	}
}
