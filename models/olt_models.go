package models

import "time"

type InfoOLT struct {
	IP        string    `json:"ip"`
	Community string    `json:"community"`
	Name      string    `json:"name"`
	Location  string    `json:"location"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type OLT struct {
	IP        string    `json:"ip"`
	Community string    `json:"community"`
	Name      string    `json:"name"`
	Location  string    `json:"location"`
	Timeout   int       `json:"timeout"`
	Retries   int       `json:"retries"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
