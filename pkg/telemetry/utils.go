// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"log"
	"math/rand"
	"net"
	"time"
)

func jitter(t time.Duration) time.Duration {
	nanos := rand.NormFloat64()*float64(t/4) + float64(t)
	if nanos <= 0 {
		nanos = 1
	}
	return time.Duration(nanos)
}

// DefaultInstanceId will return the first non-nil mac address if possible,
// unknown otherwise.
func DefaultInstanceId() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("failed to determine default instance id: %v", err)
		return "unknown"
	}
	for _, iface := range ifaces {
		if iface.HardwareAddr != nil {
			return iface.HardwareAddr.String()
		}
	}
	return "unknown"
}
