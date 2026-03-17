package models

import "time"

type Ont struct {
	Time             time.Time `json:"time"`
	GponIdx          int       `json:"gpon_idx"`
	GponInterface    string    `json:"gpon_interface"`

	OntIdx           int       `json:"ont_idx"`
	Despt            string    `json:"desp"`
	SerialNumber     string    `json:"serial_number"`
	LineProfName     string    `json:"line_profile_name"`
	ControlRanging   int32     `json:"olt_distance"`
	ControlRunStatus int32     `json:"status"`
	Temperature      int32     `json:"temperature"`
	Tx               int32     `json:"tx_power"`
	Rx               int32     `json:"rx_power"`
	BytesIn          uint64    `json:"bytes_in"`
	BytesOut         uint64    `json:"bytes_out"`
}
