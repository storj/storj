// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals

import "storj.io/storj/pkg/storj"

// Config for referrals service.
type Config struct {
	ReferralManagerURL storj.NodeURL
}
