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

func NewIPMapper(dbPath string) (*IPMapper, error) {
	reader, err := maxminddb.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &IPMapper{
		mmddbPath: dbPath,
		reader:    reader,
	}, nil
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
