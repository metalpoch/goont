package snmp

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
)

const DefaultConns = 10

type Snmp struct {
	ip        string
	community string
	retries   int
	timeout   time.Duration
	pool      chan *gosnmp.GoSNMP
}

func NewSnmp(ip, community string, retries int, timeout time.Duration, conns int) *Snmp {
	if conns <= 0 {
		conns = DefaultConns
	}
	return &Snmp{
		ip:        ip,
		community: community,
		retries:   retries,
		timeout:   timeout,
		pool:      make(chan *gosnmp.GoSNMP, conns),
	}
}

func (s *Snmp) IP() string {
	return s.ip
}

func (s *Snmp) newConn() *gosnmp.GoSNMP {
	client := &gosnmp.GoSNMP{
		Target:             s.ip,
		Port:               161,
		Community:          s.community,
		Version:            gosnmp.Version2c,
		Timeout:            s.timeout,
		Retries:            s.retries,
		MaxOids:            10,
		MaxRepetitions:     25,
		ExponentialTimeout: true,
	}

	if err := client.Connect(); err != nil {
		return nil
	}

	return client
}

func (s *Snmp) acquire() *gosnmp.GoSNMP {
	select {
	case c := <-s.pool:
		if c != nil {
			return c
		}
		return s.newConn()
	default:
		return s.newConn()
	}
}

func (s *Snmp) release(c *gosnmp.GoSNMP) {
	if c == nil {
		return
	}
	select {
	case s.pool <- c:
	default:
		c.Close()
	}
}

func (s *Snmp) Close() {
	for {
		select {
		case c := <-s.pool:
			if c != nil {
				c.Close()
			}
		default:
			return
		}
	}
}

func (s *Snmp) get(oids []string) (*gosnmp.SnmpPacket, error) {
	c := s.acquire()
	defer s.release(c)

	result, err := c.Get(oids)
	if err != nil {
		return nil, fmt.Errorf("error en SNMP Get: %v", err)
	}

	return result, nil
}

func (s *Snmp) walk(oid string) ([]gosnmp.SnmpPDU, error) {
	c := s.acquire()
	defer s.release(c)

	results, err := c.BulkWalkAll(oid)
	if err != nil {
		return nil, fmt.Errorf("error en SNMP BulkWalkAll on oid %s: %v", oid, err)
	}

	return results, nil
}

func (s *Snmp) SysInfo() (*OltInfo, error) {
	result, err := s.get([]string{sysName, sysLocation})
	if err != nil {
		return nil, err
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
	results, err := s.walk(ifName)
	if err != nil {
		return nil, err
	}

	var data []Gpon
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

	data := make(ontMeasurement)

	for i, oid := range oids {
		results, err := s.walk(oid)
		if err != nil {
			return nil, err
		}

		for _, pdu := range results {
			idx := extractOntIdx(pdu.Name)

			dataOnt, exists := data[idx]
			if !exists {
				dataOnt = ont{}
			}

			switch i {
			case 0:
				if value, ok := pdu.Value.([]byte); ok {
					dataOnt.Despt = string(value)
				}
			case 1:
				if value, ok := pdu.Value.([]byte); ok {
					dataOnt.SerialNumber = hex.EncodeToString(value)
				}
			case 2:
				if value, ok := pdu.Value.([]byte); ok {
					dataOnt.LineProfName = string(value)
				}
			case 3:
				if value, ok := toInt64(pdu.Value); ok {
					v := int32(value)
					dataOnt.ControlRanging = &v
				}
			case 4:
				if value, ok := toInt64(pdu.Value); ok {
					v := int32(value)
					dataOnt.ControlRunStatus = &v
				}
			case 5:
				if value, ok := toUint64(pdu.Value); ok {
					dataOnt.BytesOut = value
				}
			case 6:
				if value, ok := toUint64(pdu.Value); ok {
					dataOnt.BytesIn = value
				}
			case 7:
				if value, ok := toInt64(pdu.Value); ok {
					v := int32(value)
					dataOnt.Temperature = &v
				}
			case 8:
				if value, ok := toInt64(pdu.Value); ok {
					v := int32(value)
					dataOnt.Tx = &v
				}
			case 9:
				if value, ok := toInt64(pdu.Value); ok {
					v := int32(value)
					dataOnt.Rx = &v
				}
			}

			data[idx] = dataOnt
		}
	}

	return data, nil
}

func (s *Snmp) GponTraffic(gpons []Gpon) ([]GponTraffic, error) {
	want := make(map[int]bool, len(gpons))
	traffic := make(map[int]*GponTraffic, len(gpons))
	for _, g := range gpons {
		want[g.Idx] = true
		traffic[g.Idx] = &GponTraffic{Idx: g.Idx}
	}

	inResults, err := s.walk(ifHCInOctets)
	if err != nil {
		return nil, err
	}
	for _, pdu := range inResults {
		idx := extractOntIdx(pdu.Name)
		if want[idx] {
			if value, ok := toUint64(pdu.Value); ok {
				traffic[idx].BytesIn = value
			}
		}
	}

	outResults, err := s.walk(ifHCOutOctets)
	if err != nil {
		return nil, err
	}
	for _, pdu := range outResults {
		idx := extractOntIdx(pdu.Name)
		if want[idx] {
			if value, ok := toUint64(pdu.Value); ok {
				traffic[idx].BytesOut = value
			}
		}
	}

	result := make([]GponTraffic, 0, len(gpons))
	for _, g := range gpons {
		result = append(result, *traffic[g.Idx])
	}

	return result, nil
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
