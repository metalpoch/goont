package snmp

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

type Snmp struct {
	IP        string
	Community string
	Retries   int
	Timeout   time.Duration
}

func NewSnmp(ip, community string, retries int, timeout time.Duration) *Snmp {
	return &Snmp{ip, community, retries, timeout}
}

func (s *Snmp) connect() (*gosnmp.GoSNMP, error) {
	client := &gosnmp.GoSNMP{
		Target:             s.IP,
		Port:               161,
		Community:          s.Community,
		Version:            gosnmp.Version2c,
		Timeout:            s.Timeout,
		Retries:            s.Retries,
		MaxOids:            2,
		ExponentialTimeout: true,
	}

	err := client.Connect()
	if err != nil {
		return nil, fmt.Errorf("conexión fallida: %v\n", err)
	}

	return client, nil
}

func (s *Snmp) SysInfo() (*OltInfo, error) {
	client, err := s.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	result, err := client.Get([]string{sysName, sysLocation})
	if err != nil {
		return nil, fmt.Errorf("error en SNMP Get: %v", err)
	}

	var info OltInfo
	for i, variable := range result.Variables {
		switch i {
		case 0:
			if value, ok := variable.Value.([]byte); ok {
				info.SysName = string(value)
			}
		case 1:
			if value, ok := variable.Value.([]byte); ok {
				info.SysLocation = string(value)
			}
		}
	}

	return &info, nil
}

func (s *Snmp) IfNames() ([]Gpon, error) {
	client, err := s.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	var data []Gpon
	results, err := client.BulkWalkAll(ifName)
	if err != nil {
		return nil, fmt.Errorf("error en SNMP BulkWalkAll on oid %s: %v", ifName, err)
	}

	for _, pdu := range results {
		idx := extractOntIdx(pdu.Name)
		if value, ok := pdu.Value.([]byte); ok {
			str := string(value)
			if strings.HasPrefix(str, "GPON") {
				data = append(data, Gpon{idx, str})
			}
		}

	}

	return data, nil
}

func (s *Snmp) OntQuery(gpon Gpon) (ontMeasurement, error) {
	client, err := s.connect()
	if err != nil {
		return nil, err
	}
	defer client.Close()

	data := make(ontMeasurement)

	oids := []string{
		fmt.Sprintf("%s.%d", hwGponDeviceOntDespt, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponDeviceOntSerialNumber, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponDeviceOntLineProfName, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponDeviceOntControlRanging, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponDeviceOntControlRunStatus, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponOntStatisticUpBytes, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponOntStatisticDownBytes, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponOntOpticalDdmTemperature, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponOntOpticalDdmTxPower, gpon.Idx),
		fmt.Sprintf("%s.%d", hwGponOntOpticalDdmRxPower, gpon.Idx),
	}

	for i, oid := range oids {
		results, err := client.BulkWalkAll(oid)
		if err != nil {
			return nil, fmt.Errorf("error en SNMP BulkWalkAll on oid %s: %v", oid, err)
		}

		for _, pdu := range results {
			idx := extractOntIdx(pdu.Name)

			dataOnt, exists := data[idx]
			if !exists {
				dataOnt = ont{}
			}

			switch i {
			case 0: // HwGponDeviceOntDespt
				if value, ok := pdu.Value.([]byte); ok {
					dataOnt.Despt = string(value)
				}
			case 1: // HwGponDeviceOntSerialNumber
				if value, ok := pdu.Value.([]byte); ok {
					dataOnt.SerialNumber = hex.EncodeToString(value)
				}
			case 2: // HwGponDeviceOntLineProfName
				if value, ok := pdu.Value.([]byte); ok {
					dataOnt.LineProfName = string(value)
				}
			case 3: // HwGponDeviceOntControlRanging
				if value, ok := toInt64(pdu.Value); ok {
					dataOnt.ControlRanging = int32(value)
				}
			case 4: // HwGponDeviceOntControlRunStatus
				if value, ok := toInt64(pdu.Value); ok {
					dataOnt.ControlRunStatus = int32(value)
				}
			case 5: // HwGponOntStatisticUpBytes
				if value, ok := toUint64(pdu.Value); ok {
					dataOnt.BytesOut = value
				}
			case 6: // HwGponOntStatisticDownBytes
				if value, ok := toUint64(pdu.Value); ok {
					dataOnt.BytesIn = value
				}
			case 7: // hwGponOntOpticalDdmTemperature
				if value, ok := toInt64(pdu.Value); ok {
					dataOnt.Temperature = int32(value)
				}
			case 8: // HwGponOntOpticalDdmTxPower
				if value, ok := toInt64(pdu.Value); ok {
					dataOnt.Tx = int32(value)
				}
			case 9: // HwGponOntOpticalDdmRxPower
				if value, ok := toInt64(pdu.Value); ok {
					dataOnt.Rx = int32(value)
				}
			}

			data[idx] = dataOnt
		}
	}

	return data, nil
}

func extractOntIdx(oid string) int {
	var i int = strings.LastIndex(oid, ".")
	idx, err := strconv.Atoi(oid[i+1:])
	if err != nil {
		idx = -1
	}
	return idx
}

func toUint64(value any) (uint64, bool) {
	if !gosnmp.ToBigInt(value).IsUint64() {
		return 0, false
	}
	return gosnmp.ToBigInt(value).Uint64(), true
}

func toInt64(value any) (int64, bool) {
	if !gosnmp.ToBigInt(value).IsInt64() {
		return 0, false
	}
	return gosnmp.ToBigInt(value).Int64(), true
}
