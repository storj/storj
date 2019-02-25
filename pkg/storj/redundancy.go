// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

// RedundancyScheme specifies the parameters and the algorithm for redundancy
type RedundancyScheme struct {
	Algorithm RedundancyAlgorithm

	ShareSize int32

	RequiredShares int16
	RepairShares   int16
	OptimalShares  int16
	TotalShares    int16
}

// IsZero returns true if no field in the struct is set to non-zero value
func (scheme RedundancyScheme) IsZero() bool {
	return scheme == (RedundancyScheme{})
}

// RedundancyAlgorithm is the algorithm used for redundancy
type RedundancyAlgorithm byte

// List of supported redundancy algorithms
const (
	InvalidRedundancyAlgorithm = RedundancyAlgorithm(iota)
	ReedSolomon
)
