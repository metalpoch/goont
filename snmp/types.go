package snmp

const (
	// Iftable & Systable
	sysName     string = ".1.3.6.1.2.1.1.5.0"
	sysLocation string = ".1.3.6.1.2.1.1.6.0"
	ifName      string = ".1.3.6.1.2.1.31.1.1.1.1"

	// GPON port traffic (IF-MIB 64-bit counters)
	ifHCInOctets  string = ".1.3.6.1.2.1.31.1.1.1.6"
	ifHCOutOctets string = ".1.3.6.1.2.1.31.1.1.1.10"

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
	ControlRanging   *int32
	ControlRunStatus *int32
	Temperature      *int32
	Tx               *int32
	Rx               *int32
	BytesIn          uint64
	BytesOut         uint64
}

type OltInfo struct {
	SysName     string
	SysLocation string
}

type Gpon struct {
	Idx    uint64
	IfName string
}

type GponTraffic struct {
	Idx      uint64
	BytesIn  uint64
	BytesOut uint64
}

type ontMeasurement map[int]ont
