package storage

import (
	"context"
	"fmt"
	"goont/models"
	"strings"
)

func (s *Store) ListOltPorts(ctx context.Context, oltIP string) ([]models.OltPort, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT gpon_idx, gpon_interface, COUNT(*), MAX(last_seen)
		FROM onts
		WHERE olt_ip = $1
		GROUP BY gpon_idx, gpon_interface
		ORDER BY gpon_interface
	`, oltIP)
	if err != nil {
		return nil, fmt.Errorf("query olt ports: %w", err)
	}
	defer rows.Close()

	var result []models.OltPort
	for rows.Next() {
		var p models.OltPort
		if err := rows.Scan(&p.GponIdx, &p.GponInterface, &p.OntCount, &p.LastSeen); err != nil {
			return nil, fmt.Errorf("scan olt port: %w", err)
		}
		result = append(result, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}

func (s *Store) ListOnts(ctx context.Context, oltIP string, gponIdx *uint64, query string) ([]models.OntInfo, error) {
	const limit = 500

	conds := []string{"olt_ip = $1"}
	args := []any{oltIP}

	if gponIdx != nil {
		args = append(args, *gponIdx)
		conds = append(conds, fmt.Sprintf("gpon_idx = $%d", len(args)))
	}

	if query != "" {
		args = append(args, "%"+query+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf("(description ILIKE $%d OR serial_number ILIKE $%d)", n, n))
	}

	args = append(args, limit)
	sql := fmt.Sprintf(`
		SELECT gpon_idx, ont_idx, serial_number, description, line_profile, gpon_interface, last_seen
		FROM onts
		WHERE %s
		ORDER BY gpon_interface, ont_idx
		LIMIT $%d
	`, strings.Join(conds, " AND "), len(args))

	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("query onts inventory: %w", err)
	}
	defer rows.Close()

	var result []models.OntInfo
	for rows.Next() {
		var o models.OntInfo
		if err := rows.Scan(
			&o.GponIdx, &o.OntIdx, &o.SerialNumber, &o.Desp, &o.LineProfName, &o.GponInterface, &o.LastSeen,
		); err != nil {
			return nil, fmt.Errorf("scan ont info: %w", err)
		}
		result = append(result, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return result, nil
}
