// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"storj.io/storj/private/api"
)

// API represents specific API's configuration.
type API struct {
	Version        string
	Description    string
	PackageName    string
	Auth           api.Auth
	EndpointGroups []*EndpointGroup
}

// Group adds new endpoints group to API.
func (a *API) Group(name, prefix string) *EndpointGroup {
	group := &EndpointGroup{
		Name:   name,
		Prefix: prefix,
	}

	a.EndpointGroups = append(a.EndpointGroups, group)

	return group
}
