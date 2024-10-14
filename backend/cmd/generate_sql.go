package main

import (
	"fmt"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=localhost dbname=gen_sql sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	fmt.Printf("conn dev db err: %+v\n", err)
	err = db.AutoMigrate(
		&backend.AggregateReport{},
		&backend.AggregateReportRecord{},
		&backend.POReason{},
		&backend.DKIMAuthResult{},
		&backend.SPFAuthResult{},
	)
	fmt.Printf("create table in dev db err: %+v\n", err)
}
