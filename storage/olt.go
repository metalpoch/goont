package storage

import (
	"context"
	"fmt"
	"goont/models"

	"github.com/jackc/pgx/v5"
)

func (s *Store) InsertOLT(ctx context.Context, olt models.OLT) error {
	timeoutSec := olt.Timeout
	if timeoutSec == 0 {
		timeoutSec = 60
	}
	retries := olt.Retries
	if retries == 0 {
		retries = 3
	}

	_, err := s.pool.Exec(ctx, `
		INSERT INTO olts (ip, community, name, location, snmp_timeout, snmp_retries)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT(ip) DO UPDATE SET
			community = EXCLUDED.community,
			name = EXCLUDED.name,
			location = EXCLUDED.location,
			snmp_timeout = EXCLUDED.snmp_timeout,
			snmp_retries = EXCLUDED.snmp_retries,
			updated_at = now()
	`, olt.IP, olt.Community, olt.Name, olt.Location, timeoutSec, retries)
	if err != nil {
		return fmt.Errorf("insert olt: %w", err)
	}

	return nil
}

func (s *Store) GetOLTByID(ctx context.Context, ip string) (*models.InfoOLT, error) {
	var olt models.InfoOLT
	err := s.pool.QueryRow(ctx, `
		SELECT ip, community, name, location, created_at, updated_at
		FROM olts
		WHERE ip = $1
	`, ip).Scan(&olt.IP, &olt.Community, &olt.Name, &olt.Location, &olt.CreatedAt, &olt.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("query olt by ip: %w", err)
	}

	return &olt, nil
}

func (s *Store) GetInfoOLTs(ctx context.Context) ([]models.InfoOLT, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ip, community, name, location, created_at, updated_at
		FROM olts
		ORDER BY ip
	`)
	if err != nil {
		return nil, fmt.Errorf("query olts: %w", err)
	}
	defer rows.Close()

	var olts []models.InfoOLT
	for rows.Next() {
		var olt models.InfoOLT
		if err := rows.Scan(&olt.IP, &olt.Community, &olt.Name, &olt.Location, &olt.CreatedAt, &olt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan olt: %w", err)
		}
		olts = append(olts, olt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	if olts == nil {
		return []models.InfoOLT{}, nil
	}
	return olts, nil
}

func (s *Store) GetOLTs(ctx context.Context) ([]models.OLT, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ip, community, name, location, snmp_timeout, snmp_retries, created_at, updated_at
		FROM olts
		ORDER BY ip
	`)
	if err != nil {
		return nil, fmt.Errorf("query olts: %w", err)
	}
	defer rows.Close()

	var olts []models.OLT
	for rows.Next() {
		var olt models.OLT
		if err := rows.Scan(&olt.IP, &olt.Community, &olt.Name, &olt.Location, &olt.Timeout, &olt.Retries, &olt.CreatedAt, &olt.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan olt: %w", err)
		}
		olts = append(olts, olt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	if olts == nil {
		return []models.OLT{}, nil
	}
	return olts, nil
}

func (s *Store) DeleteOLT(ctx context.Context, ip string) error {
	_, err := s.pool.Exec(ctx, "DELETE FROM olts WHERE ip = $1", ip)
	if err != nil {
		return fmt.Errorf("delete olt: %w", err)
	}
	return nil
}
