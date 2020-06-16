package objectmap_test

import (
	"fmt"
	"log"
	"testing"

	"storj.io/storj/satellite/objectmap"
)

func Test_Lookup(t *testing.T) {
	mapper, err := objectmap.NewIPMapper("/home/fadila/dev/GeoLite2-City_20200609/GeoLite2-City.mmdb")

	if err != nil {
		log.Fatal(err)
	}
	defer mapper.Close()

	record, err := mapper.GetIPInfos("8.8.8.8")

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Country ISO Code:", record.Country.IsoCode)
	fmt.Println("ZIP Code: ", record.Postal.Code)
	fmt.Println("(Latitude , Longitude): (", record.Location.Latitude, ",", record.Location.Longitude, ")")
}
