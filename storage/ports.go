package storage

import (
	"context"
	"fmt"
	"goont/models"
	"goont/traffic"
	"time"
)

// PortsTraffic returns the traffic series of every GPON port of an OLT in a
// single query. The dashboard used to request one URL per port; each of those
// requests scanned the same chunk range, so aggregating here removes N-1 round
// trips and N-1 sets of buffers.
//
// Each port keeps exactly the intervals that GponTrafficData would return for
// it: same LAG window, same 2.5 Gbps filter, same status counters and same
// count_unmeasured semantics.
func (s *Store) PortsTraffic(ctx context.Context, oltIP string, from, to time.Time) ([]models.GponPortTraffic, error) {
	rows, err := s.pool.Query(ctx, `
		WITH s AS (
			SELECT time, gpon_idx, bytes_in, bytes_out,
			       count_active, count_inactive, count_error,
			       LAG(bytes_in)  OVER w AS prev_in,
			       LAG(bytes_out) OVER w AS prev_out,
			       LAG(time)      OVER w AS prev_time
			FROM gpon_measurements
			WHERE olt_ip = $1 AND time >= $2 AND time <= $3
			WINDOW w AS (PARTITION BY gpon_idx ORDER BY time)
		), t AS (
			SELECT time, gpon_idx,
			       bytes_in - prev_in AS din,
			       bytes_out - prev_out AS dout,
			       round((bytes_in - prev_in) * 8 / EXTRACT(EPOCH FROM time - prev_time)::numeric, 2)::float8 AS bps_in,
			       round((bytes_out - prev_out) * 8 / EXTRACT(EPOCH FROM time - prev_time)::numeric, 2)::float8 AS bps_out,
			       count_active, count_inactive, count_error
			FROM s
			WHERE prev_time IS NOT NULL AND bytes_in >= prev_in AND bytes_out >= prev_out
			  AND (bytes_in - prev_in) * 8 / EXTRACT(EPOCH FROM time - prev_time) <= $4
			  AND (bytes_out - prev_out) * 8 / EXTRACT(EPOCH FROM time - prev_time) <= $4
		), inv AS (
			SELECT gpon_idx, COUNT(*) AS total, MIN(gpon_interface) AS gpon_interface
			FROM onts
			WHERE olt_ip = $1
			GROUP BY gpon_idx
		)
		SELECT t.gpon_idx, COALESCE(inv.gpon_interface, ''),
		       t.time, t.din, t.dout, t.bps_in, t.bps_out,
		       t.count_active, t.count_inactive, t.count_error,
		       GREATEST(COALESCE(inv.total, 0) - t.count_active - t.count_inactive - t.count_error, 0)
		FROM t
		LEFT JOIN inv USING (gpon_idx)
		ORDER BY t.gpon_idx, t.time
	`, oltIP, from, to, traffic.MaxGponBps)
	if err != nil {
		return nil, fmt.Errorf("query ports traffic: %w", err)
	}
	defer rows.Close()

	var result []models.GponPortTraffic
	idx := -1

	for rows.Next() {
		var gponIdx uint64
		var ifName string
		var m models.GponMeasurement
		var din, dout int64

		if err := rows.Scan(
			&gponIdx, &ifName,
			&m.Time, &din, &dout, &m.BpsIn, &m.BpsOut,
			&m.CountActive, &m.CountInactive, &m.CountError, &m.CountUnmeasured,
		); err != nil {
			return nil, fmt.Errorf("scan ports traffic: %w", err)
		}

		if idx < 0 || result[idx].GponIdx != gponIdx {
			result = append(result, models.GponPortTraffic{
				GponIdx:       gponIdx,
				GponInterface: ifName,
			})
			idx = len(result) - 1
		}

		m.GponInterface = ifName
		m.BytesIn = uint64(din)
		m.BytesOut = uint64(dout)
		result[idx].Measurements = append(result[idx].Measurements, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}
