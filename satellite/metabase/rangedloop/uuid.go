// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"encoding/binary"

	"storj.io/common/uuid"
)

// UUIDRange describes a range of UUID values.
// Start and End can be open-ended.
type UUIDRange struct {
	Start *uuid.UUID
	End   *uuid.UUID
}

// CreateUUIDRanges splits up the entire 128-bit UUID range into equal parts.
func CreateUUIDRanges(nRanges uint32) ([]UUIDRange, error) {
	boundaries, err := CreateUUIDBoundaries(nRanges)
	if err != nil {
		return nil, err
	}

	return createUUIDRangesFromBoundaries(boundaries), nil
}

func createUUIDRangesFromBoundaries(boundaries []uuid.UUID) []UUIDRange {
	result := []UUIDRange{}

	for i := 0; i <= len(boundaries); i++ {
		uuidRange := UUIDRange{}

		if i != 0 {
			uuidRange.Start = &boundaries[i-1]
		}

		if i != len(boundaries) {
			uuidRange.End = &boundaries[i]
		}

		result = append(result, uuidRange)
	}

	return result
}

// CreateUUIDBoundaries splits up the entire 128-bit UUID range into equal parts.
func CreateUUIDBoundaries(nRanges uint32) ([]uuid.UUID, error) {
	if nRanges == 0 {
		// every time this line is executed a mathematician feels a disturbance in the force
		nRanges = 1
	}

	increment := uint32(1 << 32 / uint64(nRanges))

	result := []uuid.UUID{}

	for i := uint32(1); i < nRanges; i++ {
		topBits := i * increment

		newUuid, err := MakeUUIDWithTopBits(topBits)
		if err != nil {
			return nil, err
		}

		result = append(result, newUuid)
	}

	return result, nil
}

// MakeUUIDWithTopBits creates a zeroed UUID with the top 32 bits set from the input.
// Technically the result is not a UUID since it doesn't have the version and variant bits set.
func MakeUUIDWithTopBits(topBits uint32) (uuid.UUID, error) {
	bytes := make([]byte, 16)
	binary.BigEndian.PutUint32(bytes, topBits)

	return uuid.FromBytes(bytes)
}
