package objectmap

import (
	"net"

	"github.com/oschwald/maxminddb-golang"
)

type IPInfos struct {
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
	Postal struct {
		Code string `maxminddb:"code"`
	} `maxminddb:"postal"`
	Country struct {
		IsoCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

type IPMapper struct {
	mmddbPath string
	reader    *maxminddb.Reader
}

func NewIPMapper(dbPath string) (mapper IPMapper) {
	mapper.mmddbPath = dbPath
	return mapper
}

func (mapper *IPMapper) Init() (err error) {
	mapper.reader, err = maxminddb.Open(mapper.mmddbPath)
	return err
}

func (mapper *IPMapper) Close() (err error) {
	if mapper.reader != nil {
		return mapper.reader.Close()
	}
	return nil
}

func (mapper *IPMapper) GetIPInfos(ipAddress string) (record IPInfos, err error) {
	ip := net.ParseIP(ipAddress)
	err = mapper.reader.Lookup(ip, &record)
	return
}
