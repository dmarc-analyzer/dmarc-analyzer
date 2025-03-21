// Package util provides utility functions for the DMARC analyzer.
// It includes functions for domain processing, date handling, and other common operations.
package util

import (
	"log"
	"time"
)

// ParseDate converts string date representations to Unix timestamps.
// This function is used to parse date ranges for DMARC report queries.
// It handles RFC3339Nano formatted dates and provides fallback values if parsing fails.
//
// If startDate parsing fails, it defaults to 30 days before the current time.
// If endDate parsing fails, it defaults to the current time.
// If endDate is in the future, it's capped to the current time.
//
// Parameters:
//   - startDate: The start date as a string in RFC3339Nano format
//   - endDate: The end date as a string in RFC3339Nano format
//
// Returns:
//   - int64: Unix timestamp for the start date
//   - int64: Unix timestamp for the end date
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
