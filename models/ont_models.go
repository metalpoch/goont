package models

import "time"

type Ont struct {
	Time             time.Time
	GponIdx          int
	GponInterface    string
	OntIdx           int
	Despt            string
	SerialNumber     string
	LineProfName     string
	ControlRanging   *int32
	ControlRunStatus *int32
	Temperature      *int32
	Tx               *int32
	Rx               *int32
	BytesIn          uint64
	BytesOut         uint64
}

type GponSample struct {
	Time     time.Time
	GponIdx  int
	BytesIn  uint64
	BytesOut uint64
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

type GponMeasurement struct {
	Time          time.Time `json:"time"`
	GponInterface string    `json:"gpon_interface"`
	BytesIn       uint64    `json:"bytes_in"`
	BytesOut      uint64    `json:"bytes_out"`
	BpsIn         float64   `json:"bps_in"`
	BpsOut        float64   `json:"bps_out"`
	CountActive   int       `json:"count_active"`
	CountInactive int       `json:"count_inactive"`
	CountError    int       `json:"count_error"`
}

type OltMeasurement struct {
	Time          time.Time `json:"time"`
	BytesIn       uint64    `json:"bytes_in"`
	BytesOut      uint64    `json:"bytes_out"`
	BpsIn         float64   `json:"bps_in"`
	BpsOut        float64   `json:"bps_out"`
	CountActive   int       `json:"count_active"`
	CountInactive int       `json:"count_inactive"`
	CountError    int       `json:"count_error"`
}
