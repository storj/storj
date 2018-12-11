// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satelliteql

import (
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/storj/pkg/satellite"
)

type withIDFieldResolver func(p graphql.ResolveParams, id *uuid.UUID) (interface{}, error)

// resolveWithID takes id field and parse it to *uuid.UUID and pass it to inner resolve function
func resolveWithID(field string, resolver withIDFieldResolver) graphql.FieldResolveFn {
	return func(p graphql.ResolveParams) (interface{}, error) {
		var id *uuid.UUID

		idStr, ok := p.Args[field].(string)
		if !ok {
			return resolver(p, nil)
		}

		id, _ = uuid.Parse(idStr)
		return resolver(p, id)
	}
}

// resolveWithAuthID tries to take id of authorized user if id is nil
func resolveWithAuthID(field string, resolver withIDFieldResolver) graphql.FieldResolveFn {
	return resolveWithID(field, func(p graphql.ResolveParams, id *uuid.UUID) (interface{}, error) {
		if id != nil {
			return resolver(p, id)
		}

		auth, err := satellite.GetAuth(p.Context)
		if err != nil {
			return nil, err
		}

		return resolver(p, &auth.User.ID)
	})
}
