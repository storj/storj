package objectmap

import (
	"math/rand"
	"net"

	"github.com/oschwald/maxminddb-golang"
	"github.com/zeebo/errs"
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

func (mapper *IPMapper) GetIPInfos(ipAddress string) (_ *IPInfos, err error) {
	var ip string
	ip, _, err = net.SplitHostPort(ipAddress)
	if err != nil {
		ip = ipAddress // assume it had no port
	}

	// TODO(isaac): remove this after demo
	if ip == "127.0.0.1" {
		ip = randomIPs[rand.Int63n(int64(len(randomIPs)))]
	}

	var record IPInfos
	parsed := net.ParseIP(ip)
	// TODO(isaac): If it's nil, do we want to skip it, or return an error?
	if parsed == nil {
		return nil, errs.New("invalid IP address: %s", ipAddress)
	}

	err = mapper.reader.Lookup(parsed, &record)
	return &record, err
}

var randomIPs = []string{
	"56.238.100.218",
	"24.33.93.187",
	"104.51.87.191",
	"201.109.136.196",
	"94.92.86.103",
	"177.77.119.227",
	"67.90.36.132",
	"193.35.247.13",
	"72.181.6.192",
	"216.217.10.148",
	"145.39.154.239",
	"42.19.238.170",
	"115.180.22.69",
	"83.92.73.232",
	"73.183.100.16",
	"186.182.82.204",
	"49.107.95.105",
	"62.12.56.246",
	"58.179.41.243",
	"158.130.92.91",
	"202.217.155.131",
	"77.230.19.111",
	"84.11.123.78",
	"65.129.106.98",
	"143.176.240.162",
	"45.88.207.85",
	"27.246.240.33",
	"89.85.122.195",
	"43.226.23.104",
	"138.123.227.79",
	"49.36.238.23",
	"126.71.4.160",
	"107.18.79.249",
	"65.68.95.31",
	"100.10.31.4",
	"78.101.194.100",
	"89.96.38.178",
	"160.10.31.249",
	"54.80.213.116",
	"140.94.231.180",
	"181.214.125.199",
	"5.145.115.68",
	"98.234.91.81",
	"80.28.98.67",
	"179.144.11.231",
	"99.170.245.210",
	"143.50.142.195",
	"115.53.206.240",
	"81.14.160.7",
	"12.96.116.247",
	"12.184.127.105",
	"92.157.106.255",
	"189.211.94.31",
	"211.202.179.9",
	"136.199.204.234",
	"177.239.191.156",
	"39.194.244.23",
	"42.101.217.2",
	"93.91.231.155",
	"107.28.234.65",
	"93.11.159.30",
	"177.232.26.50",
	"65.82.74.102",
	"219.156.5.226",
	"18.14.119.205",
	"150.229.217.135",
	"204.188.96.0",
	"17.115.26.132",
	"84.65.4.63",
	"202.14.50.161",
	"62.39.247.25",
	"99.3.102.1",
	"201.113.60.17",
	"12.38.6.36",
	"209.250.138.223",
	"167.62.212.22",
	"58.156.82.85",
	"67.247.31.141",
	"114.191.204.180",
	"14.222.93.176",
	"119.231.245.233",
	"155.143.97.131",
	"161.107.10.171",
	"68.212.91.105",
	"195.218.65.233",
	"203.45.242.6",
}
