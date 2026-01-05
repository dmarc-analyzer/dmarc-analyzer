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
//   - startDate: The start date pointer in RFC3339Nano format (nil for default)
//   - endDate: The end date pointer in RFC3339Nano format (nil for default)
//
// Returns:
//   - int64: Unix timestamp for the start date
//   - int64: Unix timestamp for the end date
func ParseDate(startDate, endDate *string) (int64, int64) {
	now := time.Now().UTC()

	tStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -30)
	if startDate != nil && *startDate != "" {
		parsedStart, err := parseDateInput(*startDate, false)
		if err != nil {
			log.Println("ERROR reading start time", err)
		} else {
			tStart = parsedStart
		}
	}

	tEnd := now
	if endDate != nil && *endDate != "" {
		parsedEnd, err := parseDateInput(*endDate, true)
		if err != nil {
			log.Println("ERROR reading end time", err)
		} else {
			tEnd = parsedEnd
		}
	}
	if tEnd.After(now) {
		tEnd = now
	}

	start := tStart.Unix()
	end := tEnd.Unix()

	return start, end
}

func parseDateInput(value string, isEnd bool) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return parsed, nil
	}

	parsed, err = time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}

	if isEnd {
		return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 23, 59, 59, 0, time.UTC), nil
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, time.UTC), nil
}
