package storage

import (
	"context"
	"errors"
	"fmt"
	"goont/models"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Store) OntTraffic(ctx context.Context, oltIP string, gponIdx, ontIdx int, from, to time.Time) ([]models.OntMeasurement, error) {
	rows, err := s.pool.Query(ctx, `
		WITH s AS (
			SELECT time, bytes_in, bytes_out, run_status, olt_distance, temperature, tx_power, rx_power,
			       serial_number, description, line_profile,
			       LAG(bytes_in)  OVER w AS prev_in,
			       LAG(bytes_out) OVER w AS prev_out,
			       LAG(time)      OVER w AS prev_time
			FROM ont_measurements
			WHERE olt_ip = $1 AND gpon_idx = $2 AND ont_idx = $3 AND time >= $4 AND time <= $5
			WINDOW w AS (ORDER BY time)
		)
		SELECT time, din, dout, bps_in, bps_out, run_status, olt_distance, temperature, tx_power, rx_power,
		       serial_number, description, line_profile
		FROM (
			SELECT time,
			       bytes_in - prev_in AS din,
			       bytes_out - prev_out AS dout,
			       round((bytes_in - prev_in) * 8 / EXTRACT(EPOCH FROM time - prev_time)::numeric, 2)::float8 AS bps_in,
			       round((bytes_out - prev_out) * 8 / EXTRACT(EPOCH FROM time - prev_time)::numeric, 2)::float8 AS bps_out,
			       run_status, olt_distance, temperature, tx_power, rx_power,
			       serial_number, description, line_profile
			FROM s
			WHERE prev_time IS NOT NULL AND bytes_in >= prev_in AND bytes_out >= prev_out
		) x
		ORDER BY time
	`, oltIP, gponIdx, ontIdx, from, to)
	if err != nil {
		return nil, fmt.Errorf("query ont traffic: %w", err)
	}
	defer rows.Close()

	var result []models.OntMeasurement
	for rows.Next() {
		var m models.OntMeasurement
		var din, dout int64
		var runStatus, oltDistance, temperature, tx, rx *int32

		if err := rows.Scan(
			&m.Time, &din, &dout, &m.BpsIn, &m.BpsOut,
			&runStatus, &oltDistance, &temperature, &tx, &rx,
			&m.SerialNumber, &m.DNI, &m.Plan,
		); err != nil {
			return nil, fmt.Errorf("scan ont traffic: %w", err)
		}

		m.BytesIn = uint64(din)
		m.BytesOut = uint64(dout)
		if runStatus != nil {
			m.Status = int8(*runStatus)
		}
		if temperature != nil {
			m.Temperature = int8(*temperature)
		}
		if oltDistance != nil {
			m.OltDistance = int16(*oltDistance)
		}
		if tx != nil {
			m.Tx = float64(*tx) / 100
		}
		if rx != nil {
			m.Rx = float64(*rx) / 100
		}

		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

func (s *Store) GponTrafficData(ctx context.Context, oltIP string, gponIdx int, from, to time.Time) ([]models.GponMeasurement, error) {
	var ifName string
	err := s.pool.QueryRow(ctx, `
		SELECT gpon_interface FROM onts WHERE olt_ip = $1 AND gpon_idx = $2 LIMIT 1
	`, oltIP, gponIdx).Scan(&ifName)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("query gpon interface: %w", err)
	}

	rows, err := s.pool.Query(ctx, `
		WITH s AS (
			SELECT time, bytes_in, bytes_out,
			       LAG(bytes_in)  OVER w AS prev_in,
			       LAG(bytes_out) OVER w AS prev_out,
			       LAG(time)      OVER w AS prev_time
			FROM gpon_measurements
			WHERE olt_ip = $1 AND gpon_idx = $2 AND time >= $3 AND time <= $4
			WINDOW w AS (ORDER BY time)
		), t AS (
			SELECT time,
			       bytes_in - prev_in AS din,
			       bytes_out - prev_out AS dout,
			       round((bytes_in - prev_in) * 8 / EXTRACT(EPOCH FROM time - prev_time)::numeric, 2)::float8 AS bps_in,
			       round((bytes_out - prev_out) * 8 / EXTRACT(EPOCH FROM time - prev_time)::numeric, 2)::float8 AS bps_out
			FROM s
			WHERE prev_time IS NOT NULL AND bytes_in >= prev_in AND bytes_out >= prev_out
		)
		SELECT t.time, t.din, t.dout, t.bps_in, t.bps_out,
		       COALESCE(c.active, 0), COALESCE(c.inactive, 0), COALESCE(c.error, 0)
		FROM t
		LEFT JOIN (
			SELECT time,
			       COUNT(*) FILTER (WHERE run_status = 1)  AS active,
			       COUNT(*) FILTER (WHERE run_status = 2)  AS inactive,
			       COUNT(*) FILTER (WHERE run_status = -1) AS error
			FROM ont_measurements
			WHERE olt_ip = $1 AND gpon_idx = $2 AND time >= $3 AND time <= $4
			GROUP BY time
		) c ON c.time = t.time
		ORDER BY t.time
	`, oltIP, gponIdx, from, to)
	if err != nil {
		return nil, fmt.Errorf("query gpon traffic: %w", err)
	}
	defer rows.Close()

	var result []models.GponMeasurement
	for rows.Next() {
		var m models.GponMeasurement
		var din, dout int64

		if err := rows.Scan(
			&m.Time, &din, &dout, &m.BpsIn, &m.BpsOut,
			&m.CountActive, &m.CountInactive, &m.CountError,
		); err != nil {
			return nil, fmt.Errorf("scan gpon traffic: %w", err)
		}

		m.GponInterface = ifName
		m.BytesIn = uint64(din)
		m.BytesOut = uint64(dout)

		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

func (s *Store) OltTraffic(ctx context.Context, oltIP string, from, to time.Time) ([]models.OltMeasurement, error) {
	rows, err := s.pool.Query(ctx, `
		WITH s AS (
			SELECT time, gpon_idx, bytes_in, bytes_out,
			       LAG(bytes_in)  OVER (PARTITION BY gpon_idx ORDER BY time) AS prev_in,
			       LAG(bytes_out) OVER (PARTITION BY gpon_idx ORDER BY time) AS prev_out,
			       LAG(time)      OVER (PARTITION BY gpon_idx ORDER BY time) AS prev_time
			FROM gpon_measurements
			WHERE olt_ip = $1 AND time >= $2 AND time <= $3
		), t AS (
			SELECT time,
			       bytes_in - prev_in AS din,
			       bytes_out - prev_out AS dout,
			       round((bytes_in - prev_in) * 8 / EXTRACT(EPOCH FROM time - prev_time)::numeric, 2)::float8 AS bps_in,
			       round((bytes_out - prev_out) * 8 / EXTRACT(EPOCH FROM time - prev_time)::numeric, 2)::float8 AS bps_out
			FROM s
			WHERE prev_time IS NOT NULL AND bytes_in >= prev_in AND bytes_out >= prev_out
		)
		SELECT agg.time, agg.din, agg.dout, agg.bps_in, agg.bps_out,
		       COALESCE(c.active, 0), COALESCE(c.inactive, 0), COALESCE(c.error, 0)
		FROM (
			SELECT time,
			       SUM(din)::bigint AS din,
			       SUM(dout)::bigint AS dout,
			       SUM(bps_in) AS bps_in,
			       SUM(bps_out) AS bps_out
			FROM t
			GROUP BY time
		) agg
		LEFT JOIN (
			SELECT time,
			       COUNT(*) FILTER (WHERE run_status = 1)  AS active,
			       COUNT(*) FILTER (WHERE run_status = 2)  AS inactive,
			       COUNT(*) FILTER (WHERE run_status = -1) AS error
			FROM ont_measurements
			WHERE olt_ip = $1 AND time >= $2 AND time <= $3
			GROUP BY time
		) c ON c.time = agg.time
		ORDER BY agg.time
	`, oltIP, from, to)
	if err != nil {
		return nil, fmt.Errorf("query olt traffic: %w", err)
	}
	defer rows.Close()

	var result []models.OltMeasurement
	for rows.Next() {
		var m models.OltMeasurement
		var din, dout int64

		if err := rows.Scan(
			&m.Time, &din, &dout, &m.BpsIn, &m.BpsOut,
			&m.CountActive, &m.CountInactive, &m.CountError,
		); err != nil {
			return nil, fmt.Errorf("scan olt traffic: %w", err)
		}

		m.BytesIn = uint64(din)
		m.BytesOut = uint64(dout)

		result = append(result, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}
