package handlers

import (
	"fmt"
	"strconv"
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

func parseGponIdx(s string) (uint64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return uint64(uint32(n)), nil
	}
	return uint64(n), nil
}
