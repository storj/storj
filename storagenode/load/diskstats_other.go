// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux

package load

import "fmt"

type rawDiskStats struct {
	ReadsCompleted  uint64
	ReadsMerged     uint64
	SectorsRead     uint64
	MsReading       uint64
	WritesCompleted uint64
	WritesMerged    uint64
	SectorsWritten  uint64
	MsWriting       uint64
	IOsInProgress   uint64
	MsDoingIO       uint64
	WeightedMsIO    uint64
}

func readDiskStats(_ string) (rawDiskStats, error) {
	return rawDiskStats{}, fmt.Errorf("disk stats not supported on this platform")
}

func deviceNameFromPath(_ string) (string, error) {
	return "", fmt.Errorf("device name resolution not supported on this platform")
}
