// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

// API represents specific API's configuration.
type API struct {
	Version        string
	Description    string
	EndpointGroups []*EndpointGroup
}

// New creates new API with specific configuration.
func New(version, description string) *API {
	return &API{
		Version:     version,
		Description: description,
	}
}

// Group adds new endpoints group to API.
func (a *API) Group(name, prefix string) *EndpointGroup {
	group := &EndpointGroup{
		Name:      name,
		Prefix:    prefix,
		Endpoints: make(map[PathMethod]*Endpoint),
	}

	a.EndpointGroups = append(a.EndpointGroups, group)

	return group
}
