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

type GponMeasurement struct {
	Time          time.Time `json:"time"`
	GponIdx       int       `json:"gpon_idx"`
	GponInterface string    `json:"gpon_interface"`
	BpsIn         float64   `json:"bps_in"`
	BpsOut        float64   `json:"bps_out"`
	BytesIn       uint64    `json:"bytes_in"`
	BytesOut      uint64    `json:"bytes_out"`
}

type OntMeasurement struct {
	Time         time.Time `json:"time"`
	Status       int8      `json:"status"`
	Temperature  int8      `json:"temperature"`
	OltDistance  int16     `json:"olt_distance"`
	Tx           float64   `json:"tx_power"`
	Rx           float64   `json:"rx_power"`
	BpsIn        float64   `json:"bps_in"`
	BpsOut       float64   `json:"bps_out"`
	BytesIn      uint64    `json:"bytes_in"`
	BytesOut     uint64    `json:"bytes_out"`
	DNI          string    `json:"desp"`
	SerialNumber string    `json:"serial_number"`
	Plan         string    `json:"plan"`
}
type GponGrouped map[int][]Ont              // GponIdx as key
type OntGrouped map[uint8][]Ont             // OntIdx as key
type OntResponse map[uint8][]OntMeasurement // OntIdx as key
