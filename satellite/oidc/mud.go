// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package oidc

import "storj.io/storj/shared/mud"

// Module is a mud module definition.
func Module(ball *mud.Ball) {
	mud.Provide[*Service](ball, NewService)
	mud.View[DB, OAuthTokens](ball, func(db DB) OAuthTokens {
		return db.OAuthTokens()
	})
}
