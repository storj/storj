// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/satellite/console"
)

// uuidIDAuthFallback returns auth user id if no id argument provided
func uuidIDAuthFallback(p graphql.ResolveParams, field string) (*uuid.UUID, error) {
	// if client passed id - parse it and return
	if idStr, ok := p.Args[field].(string); ok {
		return uuid.Parse(idStr)
	}

	// else get id of authorized user
	auth, err := console.GetAuth(p.Context)
	if err != nil {
		return nil, err
	}

	return &auth.User.ID, nil
}
