// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import "storj.io/common/storj"

// DefaultRedundancy contains default redundancy scheme.
var DefaultRedundancy = storj.RedundancyScheme{
	Algorithm:      storj.ReedSolomon,
	ShareSize:      2048,
	RequiredShares: 1,
	RepairShares:   1,
	OptimalShares:  1,
	TotalShares:    1,
}

// DefaultEncryption contains default encryption parameters.
var DefaultEncryption = storj.EncryptionParameters{
	CipherSuite: storj.EncAESGCM,
	BlockSize:   29 * 256,
}
