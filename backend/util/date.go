package util

import (
	"log"
	"time"
)

func ParseDate(startDate, endDate string) (int64, int64) {
	now := time.Now()

	tStart, err := time.Parse(time.RFC3339Nano, startDate)
	if err != nil {
		log.Println("ERROR reading start time", err)
		tStart = now.AddDate(0, 0, -30)
	}

	tEnd, err := time.Parse(time.RFC3339Nano, endDate)
	if err != nil {
		log.Println("ERROR reading end time", err)
		tEnd = now
	}
	if tEnd.After(now) {
		tEnd = now
	}

	start := tStart.Unix()
	end := tEnd.Unix()

	return start, end
}
