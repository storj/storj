package main

import (
	"flag"

	zipkin "gopkg.in/spacemonkeygo/monkit-zipkin.v2"
)

var (
	scribeAddr = flag.String("scribe_addr", "localhost:9410", "address of the scribe endpoint")
	listenAddr = flag.String("listen_addr", "localhost:9411", "address to listen for monkit-zipkin udp packets on")
)

func main() {
	flag.Parse()
	collector, err := zipkin.NewScribeCollector(*scribeAddr)
	if err != nil {
		panic(err)
	}
	panic(zipkin.RedirectPackets(*listenAddr, collector))
}
