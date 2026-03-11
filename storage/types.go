package storage

import "time"

type OLT struct {
	IP        string
	Community string
	Name      string
	Location  string
	Timeout   int
	Retries   int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type InfoOLT struct {
	IP        string
	Community string
	Name      string
	Location  string
	CreatedAt time.Time
	UpdatedAt time.Time
}
