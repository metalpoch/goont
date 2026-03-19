package handlers

import (
	"fmt"
	"goont/models"
	"math"
	"sort"
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
		if measurement.Temperature != math.MaxInt32 {
			temp = measurement.Temperature
		}
		if measurement.ControlRanging != math.MaxInt32 {
			distance = measurement.ControlRanging
		}
		if measurement.ControlRunStatus != math.MaxInt32 {
			status = measurement.ControlRunStatus
		}
		if measurement.Tx != math.MaxInt32 {
			tx = measurement.Tx
		}
		if measurement.Rx != math.MaxInt32 {
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
				if measurement.Temperature != math.MaxInt32 {
					temp = measurement.Temperature
				}
				if measurement.ControlRanging != math.MaxInt32 {
					distance = measurement.ControlRanging
				}
				if measurement.ControlRunStatus != math.MaxInt32 {
					status = measurement.ControlRunStatus
				}
				if measurement.Tx != math.MaxInt32 {
					tx = measurement.Tx
				}
				if measurement.Rx != math.MaxInt32 {
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

func processGponTraffic(measurements []models.Ont) models.GponResponse {
	// Group by (gpon_idx, ont_idx)
	grouped := make(map[int]map[int][]models.Ont)
	for _, m := range measurements {
		if _, ok := grouped[m.GponIdx]; !ok {
			grouped[m.GponIdx] = make(map[int][]models.Ont)
		}
		grouped[m.GponIdx][m.OntIdx] = append(grouped[m.GponIdx][m.OntIdx], m)
	}

	// Map to accumulate hourly traffic per GPON
	// Key: gpon_idx -> hour -> accumulator
	accum := make(map[int]map[time.Time]*models.GponMeasurement)
	// Store gpon_interface per GPON
	gponInterfaces := make(map[int]string)
	// Map to track status per ONT per hour (gpon_idx -> hour -> ont_idx -> status)
	statusMap := make(map[int]map[time.Time]map[int]int8)

	for gponIdx, ontMap := range grouped {
		for _, ontMeasurements := range ontMap {
			// Sort by time (already sorted from DB)
			for i := 1; i < len(ontMeasurements); i++ {
				prev := ontMeasurements[i-1]
				curr := ontMeasurements[i]

				// Calculate deltas
				deltaDuration := curr.Time.Sub(prev.Time)
				bytesIn := deltaBytes(prev.BytesIn, curr.BytesIn)
				bytesOut := deltaBytes(prev.BytesOut, curr.BytesOut)
				bpsIn := calculateBPS(bytesIn, deltaDuration)
				bpsOut := calculateBPS(bytesOut, deltaDuration)

				// Determine status from current measurement
				status := int8(0)
				if curr.ControlRunStatus != math.MaxInt32 {
					status = int8(curr.ControlRunStatus)
				}

				// Truncate to hour
				hour := curr.Time.Truncate(time.Hour)

				// Initialize accum maps if needed
				if _, ok := accum[gponIdx]; !ok {
					accum[gponIdx] = make(map[time.Time]*models.GponMeasurement)
				}
				if _, ok := accum[gponIdx][hour]; !ok {
					// Store gpon_interface (take from first measurement)
					if gponInterfaces[gponIdx] == "" && len(ontMeasurements) > 0 {
						gponInterfaces[gponIdx] = ontMeasurements[0].GponInterface
					}
					accum[gponIdx][hour] = &models.GponMeasurement{
						GponInterface: gponInterfaces[gponIdx],
						Time:          hour,
					}
				}

				// Add to totals
				entry := accum[gponIdx][hour]
				entry.TotalBytesIn += bytesIn
				entry.TotalBytesOut += bytesOut
				entry.TotalBpsIn += bpsIn
				entry.TotalBpsOut += bpsOut

				// Register status for this ONT at this hour
				if statusMap[gponIdx] == nil {
					statusMap[gponIdx] = make(map[time.Time]map[int]int8)
				}
				if statusMap[gponIdx][hour] == nil {
					statusMap[gponIdx][hour] = make(map[int]int8)
				}
				statusMap[gponIdx][hour][curr.OntIdx] = status
			}
		}
	}

	// Count statuses per GPON per hour
	for gponIdx, hourMap := range statusMap {
		for hour, ontStatusMap := range hourMap {
			entry := accum[gponIdx][hour]
			for _, status := range ontStatusMap {
				switch status {
				case 1:
					entry.CountActive++
				case 2:
					entry.CountInactive++
				case -1:
					entry.CountError++
				}
			}
		}
	}

	// Build map gpon_idx -> sorted list by hour
	result := make(models.GponResponse)

	for gponIdx, hourMap := range accum {
		var entries []models.GponResponse
		for _, entry := range hourMap {
			entries = append(entries, *entry)
		}
		// Sort entries by hour ascending
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Time.Before(entries[j].Time)
		})
		result[gponIdx] = entries
	}

	return result
}
