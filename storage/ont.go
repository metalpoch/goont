package storage

import (
	"context"
	"fmt"
	"goont/models"

	"github.com/jackc/pgx/v5"
)

func (s *Store) UpsertOnts(ctx context.Context, oltIP string, onts []models.Ont) error {
	if len(onts) == 0 {
		return nil
	}

	gponIdx := make([]int64, len(onts))
	ontIdx := make([]int32, len(onts))
	serial := make([]string, len(onts))
	description := make([]string, len(onts))
	lineProfile := make([]string, len(onts))
	gponInterface := make([]string, len(onts))

	for i, o := range onts {
		gponIdx[i] = int64(o.GponIdx)
		ontIdx[i] = int32(o.OntIdx)
		serial[i] = o.SerialNumber
		description[i] = o.Despt
		lineProfile[i] = o.LineProfName
		gponInterface[i] = o.GponInterface
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO onts (olt_ip, gpon_idx, ont_idx, serial_number, description, line_profile, gpon_interface, last_seen)
		SELECT $1, g, o, s, d, l, i, $2
		FROM unnest($3::bigint[], $4::int[], $5::text[], $6::text[], $7::text[], $8::text[]) AS t(g, o, s, d, l, i)
		ON CONFLICT (olt_ip, gpon_idx, ont_idx) DO UPDATE SET
			serial_number = EXCLUDED.serial_number,
			description = EXCLUDED.description,
			line_profile = EXCLUDED.line_profile,
			gpon_interface = EXCLUDED.gpon_interface,
			last_seen = EXCLUDED.last_seen
	`, oltIP, onts[0].Time, gponIdx, ontIdx, serial, description, lineProfile, gponInterface)
	if err != nil {
		return fmt.Errorf("upsert onts: %w", err)
	}

	return nil
}

func (s *Store) InsertOntMeasurements(ctx context.Context, oltIP string, onts []models.Ont) error {
	if len(onts) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(onts))
	for _, o := range onts {
		rows = append(rows, []any{
			o.Time,
			oltIP,
			int64(o.GponIdx),
			int32(o.OntIdx),
			o.SerialNumber,
			o.Despt,
			o.LineProfName,
			o.ControlRunStatus,
			o.ControlRanging,
			o.Temperature,
			o.Tx,
			o.Rx,
			int64(o.BytesIn),
			int64(o.BytesOut),
		})
	}

	cols := []string{
		"time", "olt_ip", "gpon_idx", "ont_idx", "serial_number", "description", "line_profile",
		"run_status", "olt_distance", "temperature", "tx_power", "rx_power", "bytes_in", "bytes_out",
	}

	if _, err := s.pool.CopyFrom(ctx, pgx.Identifier{"ont_measurements"}, cols, pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("copy ont measurements: %w", err)
	}

	return nil
}

func (s *Store) InsertGponMeasurements(ctx context.Context, oltIP string, samples []models.GponSample) error {
	if len(samples) == 0 {
		return nil
	}

	rows := make([][]any, 0, len(samples))
	for _, m := range samples {
		rows = append(rows, []any{
			m.Time,
			oltIP,
			int64(m.GponIdx),
			int64(m.BytesIn),
			int64(m.BytesOut),
		})
	}

	cols := []string{"time", "olt_ip", "gpon_idx", "bytes_in", "bytes_out"}

	if _, err := s.pool.CopyFrom(ctx, pgx.Identifier{"gpon_measurements"}, cols, pgx.CopyFromRows(rows)); err != nil {
		return fmt.Errorf("copy gpon measurements: %w", err)
	}

	return nil
}
