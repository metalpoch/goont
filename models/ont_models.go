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
	ControlRanging   int32
	ControlRunStatus int32
	Temperature      int32
	Tx               int32
	Rx               int32
	BytesIn          uint64
	BytesOut         uint64
}
