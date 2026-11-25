package models

import "time"

type Ont struct {
	Time             time.Time
	GponIdx          uint64
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
	// BytesInValid/BytesOutValid distinguen un contador realmente leído de uno
	// que no se pudo consultar (walk con timeout). Los inválidos se guardan
	// como NULL para que el cálculo de bps los trate como hueco, no como 0.
	BytesInValid  bool
	BytesOutValid bool
}

type GponSample struct {
	Time     time.Time
	GponIdx  uint64
	BytesIn  uint64
	BytesOut uint64
}

type OntMeasurement struct {
	Time         time.Time `json:"time"`
	Status       int8      `json:"status"`
	Temperature  *int8     `json:"temperature"`
	OltDistance  *int16    `json:"olt_distance"`
	Tx           *float64  `json:"tx_power"`
	Rx           *float64  `json:"rx_power"`
	BpsIn        float64   `json:"bps_in"`
	BpsOut       float64   `json:"bps_out"`
	BytesIn      uint64    `json:"bytes_in"`
	BytesOut     uint64    `json:"bytes_out"`
	DNI          string    `json:"desp"`
	SerialNumber string    `json:"serial_number"`
	Plan         string    `json:"plan"`
}

type GponMeasurement struct {
	Time            time.Time `json:"time"`
	GponInterface   string    `json:"gpon_interface"`
	BytesIn         uint64    `json:"bytes_in"`
	BytesOut        uint64    `json:"bytes_out"`
	BpsIn           float64   `json:"bps_in"`
	BpsOut          float64   `json:"bps_out"`
	CountActive     int       `json:"count_active"`
	CountInactive   int       `json:"count_inactive"`
	CountError      int       `json:"count_error"`
	CountUnmeasured int       `json:"count_unmeasured"`
}

type OltMeasurement struct {
	Time            time.Time `json:"time"`
	BytesIn         uint64    `json:"bytes_in"`
	BytesOut        uint64    `json:"bytes_out"`
	BpsIn           float64   `json:"bps_in"`
	BpsOut          float64   `json:"bps_out"`
	CountActive     int       `json:"count_active"`
	CountInactive   int       `json:"count_inactive"`
	CountError      int       `json:"count_error"`
	CountUnmeasured int       `json:"count_unmeasured"`
}

// GponPortTraffic groups the traffic series of one GPON port. It is the item
// type of GET /api/v1/traffic/{ip}/ports, which returns every port of an OLT in
// one response instead of one request per port.
type GponPortTraffic struct {
	GponIdx       uint64            `json:"gpon_idx"`
	GponInterface string            `json:"gpon_interface"`
	Measurements  []GponMeasurement `json:"measurements"`
}

type OltPort struct {
	GponIdx       uint64    `json:"gpon_idx"`
	GponInterface string    `json:"gpon_interface"`
	OntCount      int       `json:"ont_count"`
	LastSeen      time.Time `json:"last_seen"`
}

type OntInfo struct {
	GponIdx       uint64    `json:"gpon_idx"`
	OntIdx        int       `json:"ont_idx"`
	Desp          string    `json:"desp"`
	SerialNumber  string    `json:"serial_number"`
	LineProfName  string    `json:"line_profile"`
	GponInterface string    `json:"gpon_interface"`
	LastSeen      time.Time `json:"last_seen"`
}
