package snmp

import (
	"context"
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
		Target:         s.ip,
		Port:           161,
		Community:      s.community,
		Version:        gosnmp.Version2c,
		Timeout:        s.timeout,
		Retries:        s.retries,
		MaxOids:        10,
		MaxRepetitions: 25,
		// Sin backoff exponencial: con 15 s y 2 reintentos una consulta muerta
		// cuesta ~45 s, no 15+30+60. El escaneo tiene un deadline fijo y no
		// puede permitirse esperas crecientes por cada OID.
		ExponentialTimeout: false,
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
		idx, ok := extractOIDIndex(pdu.Name)
		if !ok {
			continue
		}
		if value, ok := pdu.Value.([]byte); ok {
			str := string(value)
			if strings.HasPrefix(str, "GPON") {
				data = append(data, Gpon{idx, str})
			}
		}
	}

	return data, nil
}

func (s *Snmp) OntQuery(ctx context.Context, gpon Gpon) (ontMeasurement, error) {
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
	var firstErr error

	for i, oid := range oids {
		if err := ctx.Err(); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			break
		}

		results, err := s.walk(oid)
		if err != nil {
			// Un OID que falla (típicamente un timeout) no debe tirar los datos
			// de los otros: se guarda lo leído, se deja NULL lo que faltó y se
			// sigue con el resto mientras el deadline del ciclo lo permita.
			if firstErr == nil {
				firstErr = err
			}
			continue
		}

		for _, pdu := range results {
			oidIdx, ok := extractOIDIndex(pdu.Name)
			if !ok {
				continue
			}
			idx := int(oidIdx)

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
					dataOnt.BytesOutValid = true
				}
			case 6:
				if value, ok := toUint64(pdu.Value); ok {
					dataOnt.BytesIn = value
					dataOnt.BytesInValid = true
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

	return data, firstErr
}

// GponTrafficPort lee los contadores de un único puerto GPON. El escaneo lo
// llama puerto a puerto para que el fallo de uno no descarte el tráfico de los
// demás (antes GponTraffic abortaba todo el OLT al primer error).
func (s *Snmp) GponTrafficPort(ctx context.Context, g Gpon) (GponTraffic, error) {
	t := GponTraffic{Idx: g.Idx}

	if err := ctx.Err(); err != nil {
		return t, err
	}
	inResults, err := s.walk(fmt.Sprintf("%s.%d", hwGponOltEthernetStatisticReceivedBytes, g.Idx))
	if err != nil {
		return t, err
	}
	for _, pdu := range inResults {
		if v, ok := toUint64(pdu.Value); ok {
			t.BytesIn += v
		}
	}

	if err := ctx.Err(); err != nil {
		return t, err
	}
	outResults, err := s.walk(fmt.Sprintf("%s.%d", hwGponOltEthernetStatisticSendBytes, g.Idx))
	if err != nil {
		return t, err
	}
	for _, pdu := range outResults {
		if v, ok := toUint64(pdu.Value); ok {
			t.BytesOut += v
		}
	}

	return t, nil
}

func extractOIDIndex(oid string) (uint64, bool) {
	i := strings.LastIndex(oid, ".")
	if i < 0 || i+1 >= len(oid) {
		return 0, false
	}
	idx, err := strconv.ParseUint(oid[i+1:], 10, 64)
	if err != nil {
		return 0, false
	}
	return idx, true
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
