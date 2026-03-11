package snmp

import "time"

const (
	// Iftable & Systable
	sysName     string = ".1.3.6.1.2.1.1.5.0"
	sysLocation string = ".1.3.6.1.2.1.1.6.0"
	ifName      string = ".1.3.6.1.2.1.31.1.1.1.1"

	// ONT queries
	hwGponDeviceOntDespt            string = ".1.3.6.1.4.1.2011.6.128.1.1.2.43.1.9"
	hwGponDeviceOntSerialNumber     string = ".1.3.6.1.4.1.2011.6.128.1.1.2.43.1.3"
	hwGponDeviceOntLineProfName     string = ".1.3.6.1.4.1.2011.6.128.1.1.2.43.1.7"
	hwGponDeviceOntControlRanging   string = ".1.3.6.1.4.1.2011.6.128.1.1.2.46.1.20"
	hwGponDeviceOntControlRunStatus string = ".1.3.6.1.4.1.2011.6.128.1.1.2.46.1.15"
	hwGponOntStatisticUpBytes       string = ".1.3.6.1.4.1.2011.6.128.1.1.4.23.1.3"
	hwGponOntStatisticDownBytes     string = ".1.3.6.1.4.1.2011.6.128.1.1.4.23.1.4"
	hwGponOntOpticalDdmTemperature  string = ".1.3.6.1.4.1.2011.6.128.1.1.2.51.1.1"
	hwGponOntOpticalDdmTxPower      string = ".1.3.6.1.4.1.2011.6.128.1.1.2.51.1.3"
	hwGponOntOpticalDdmRxPower      string = ".1.3.6.1.4.1.2011.6.128.1.1.2.51.1.4"
)

type ont struct {
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

type OltInfo struct {
	SysName     string
	SysLocation string
}

type Gpon struct {
	Idx    int
	IfName string
}

type ontMeasurement map[int]ont

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
