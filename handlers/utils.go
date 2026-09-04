package handlers

import (
	"fmt"
	"time"
)

type rangeDate struct {
	InitDate time.Time
	EndDate  time.Time
}

func parseTimeParam(param string) (time.Time, error) {
	if param == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, param)
}

func parseDate(initDateStr, endDateStr string) (rangeDate, error) {
	var dates rangeDate

	if initDateStr == "" || endDateStr == "" {
		return dates, fmt.Errorf("Both initDate and endDate must be provided when using date range")
	}

	initTime, err := parseTimeParam(initDateStr)
	if err != nil {
		return dates, fmt.Errorf("Invalid initDate format: %v", err)
	}

	endTime, err := parseTimeParam(endDateStr)
	if err != nil {
		return dates, fmt.Errorf("Invalid endDate format: %v", err)
	}

	if endTime.Before(initTime) {
		return dates, fmt.Errorf("endDate must be after initDate")
	}

	return rangeDate{initTime, endTime}, nil
}
