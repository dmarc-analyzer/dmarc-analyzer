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

		reports := backend.ParseDmarcReport(feedback)
		fmt.Printf("%+v\n", reports)

		dsn := "host=localhost user=postgres password=postgres dbname=dmarc sslmode=disable"
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
		fmt.Printf("%+v\n", err)
		result := db.Create(feedback)
		if result.Error != nil {
			fmt.Printf("%+v\n", result.Error)
		}
		for _, record := range feedback.Records {
			db.Create(record)
			for _, dkim := range record.AuthDKIM {
				db.Create(dkim)
			}
			for _, spf := range record.AuthSPF {
				db.Create(spf)
			}
			for _, reason := range record.POReason {
				db.Create(reason)
			}
		}
		result2 := db.Create(reports)
		if result2.Error != nil {
			fmt.Printf("%+v\n", result2.Error)
		}
	}

}
