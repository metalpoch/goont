package handlers

import (
	"fmt"
	"goont/models"
	"math"
	"sync"
	"time"
)

type rangeDate struct {
	InitDate time.Time
	EndDate  time.Time
}

// parseTimeParam parses a time parameter from query string.
// Supports RFC3339 format (e.g., "2006-01-02T15:04:05Z07:00").
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

func deltaBytes(oldBytes, newBytes uint64) uint64 {
	if newBytes >= oldBytes {
		return newBytes - oldBytes
	}
	return math.MaxUint64 - oldBytes + newBytes
}

func calculateBPS(deltaBytes uint64, deltaDuration time.Duration) float64 {
	if deltaDuration <= 0 {
		return 0.0
	}

	bytesPerSec := float64(deltaBytes) / deltaDuration.Seconds()
	return bytesPerSec * 8
}

func proccessOnt(measurements []models.Ont) []models.OntMeasurement {
	var response []models.OntMeasurement

	for prev, measurement := range measurements[1:] {
		var temp, distance, status, tx, rx int32
		if temp != math.MaxInt32 {
			temp = measurement.Temperature
		}
		if distance != math.MaxInt32 {
			distance = measurement.ControlRanging
		}
		if status != math.MaxInt32 {
			status = measurement.ControlRunStatus
		}
		if tx != math.MaxInt32 {
			tx = measurement.Tx
		}
		if rx != math.MaxInt32 {
			rx = measurement.Rx
		}

		deltaDuration := measurement.Time.Sub(measurements[prev].Time)
		bytesIn := deltaBytes(measurements[prev].BytesIn, measurement.BytesIn)
		bytesOut := deltaBytes(measurements[prev].BytesOut, measurement.BytesOut)
		bpsIn := calculateBPS(bytesIn, deltaDuration)
		bpsOut := calculateBPS(bytesOut, deltaDuration)

		m := models.OntMeasurement{
			Time:         measurement.Time,
			Status:       int8(status),
			Temperature:  int8(temp),
			OltDistance:  int16(distance),
			Tx:           float64(tx) / 100,
			Rx:           float64(rx) / 100,
			DNI:          measurement.Despt,
			SerialNumber: measurement.SerialNumber,
			Plan:         measurement.LineProfName,
			BytesIn:      bytesIn,
			BytesOut:     bytesOut,
			BpsIn:        bpsIn,
			BpsOut:       bpsOut,
		}

		response = append(response, m)
	}

	return response
}

func proccessGroupedOnt(measurements []models.Ont) models.OntResponse {
	groups := make(models.OntGrouped)
	response := make(models.OntResponse)

	for _, m := range measurements {
		id := uint8(m.OntIdx)
		r := groups[id]
		r = append(r, m)
		groups[id] = r
	}

	var wg sync.WaitGroup
	var mux sync.Mutex

	for idx, measurements := range groups {
		wg.Go(func() {
			ont := response[idx]
			for prev, measurement := range measurements[1:] {
				var temp, distance, status, tx, rx int32
				if temp != math.MaxInt32 {
					temp = measurement.Temperature
				}
				if distance != math.MaxInt32 {
					distance = measurement.ControlRanging
				}
				if status != math.MaxInt32 {
					status = measurement.ControlRunStatus
				}
				if tx != math.MaxInt32 {
					tx = measurement.Tx
				}
				if rx != math.MaxInt32 {
					rx = measurement.Rx
				}

				deltaDuration := measurement.Time.Sub(measurements[prev].Time)
				bytesIn := deltaBytes(measurements[prev].BytesIn, measurement.BytesIn)
				bytesOut := deltaBytes(measurements[prev].BytesOut, measurement.BytesOut)
				bpsIn := calculateBPS(bytesIn, deltaDuration)
				bpsOut := calculateBPS(bytesOut, deltaDuration)

				m := models.OntMeasurement{
					Time:         measurement.Time,
					Status:       int8(status),
					Temperature:  int8(temp),
					OltDistance:  int16(distance),
					Tx:           float64(tx) / 100,
					Rx:           float64(rx) / 100,
					DNI:          measurement.Despt,
					SerialNumber: measurement.SerialNumber,
					Plan:         measurement.LineProfName,
					BytesIn:      bytesIn,
					BytesOut:     bytesOut,
					BpsIn:        bpsIn,
					BpsOut:       bpsOut,
				}

				mux.Lock()
				ont = append(ont, m)
				mux.Unlock()
			}

			mux.Lock()
			response[idx] = ont
			mux.Unlock()
		})

	}

	wg.Wait()

	return response
}
