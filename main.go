package main

import (
	"fmt"

	"github.com/dmarc-analyzer/dmarc-analyzer/backend"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	//feedback, err := backend.ParseNewMail("dragonplus-dmarc-report", "q62t9ea2svdi6glcofhr1r9eqcp9ora5plkvnh81")
	//fmt.Printf("%+v %+v\n", feedback, err)
	dsn := "host=localhost user=postgres password=postgres dbname=dmarc sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	fmt.Printf("%+v\n", err)
	err = db.AutoMigrate(&backend.AggregateReport{})
	fmt.Printf("%+v\n", err)

}
