// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"log"
	"math/rand"
	"net"
	"time"
)

const (
	unknownInstanceID = "unknown"
)

func jitter(t time.Duration) time.Duration {
	nanos := rand.NormFloat64()*float64(t/4) + float64(t)
	if nanos <= 0 {
		nanos = 1
	}
	return time.Duration(nanos)
}

// DefaultInstanceID will return the first non-nil mac address if possible,
// unknown otherwise.
func DefaultInstanceID() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("failed to determine default instance id: %v", err)
		return unknownInstanceID
	}
	for _, iface := range ifaces {
		if iface.HardwareAddr != nil {
			return iface.HardwareAddr.String()
		}
	}
	return unknownInstanceID
}
