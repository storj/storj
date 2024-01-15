// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"

	"storj.io/storj/private/api"
)

// Settings contains relevant settings for the consumers of this service. It may contain settings
// of:
//
// - this service.
//
// - the server that exposes the service.
//
// - related Storj services (e.g. Satellite).
type Settings struct {
	Admin SettingsAdmin `json:"admin"`
}

// SettingsAdmin are the settings of this service and the server that exposes it.
type SettingsAdmin struct {
	Features FeatureFlags `json:"features"`
}

// FeatureFlags indicates what Admin service features are enabled or disabled. The features are
// usually disabled when they are not fully implemented.
type FeatureFlags struct {
	Account         AccountFlags `json:"account"`
	Project         ProjectFlags `json:"project"`
	Bucket          BucketFlags  `json:"bucket"`
	Dashboard       bool         `json:"dashboard"`
	Operator        bool         `json:"operator"` // This is the information about the logged operator
	SignOut         bool         `json:"signOut"`
	SwitchSatellite bool         `json:"switchSatellite"`
}

// AccountFlags are the feature flags related to user's accounts.
type AccountFlags struct {
	Create                 bool `json:"create"`
	Delete                 bool `json:"delete"`
	History                bool `json:"history"`
	List                   bool `json:"list"`
	Projects               bool `json:"projects"`
	Search                 bool `json:"search"`
	Suspend                bool `json:"suspend"`
	Unsuspend              bool `json:"unsuspend"`
	ResetMFA               bool `json:"resetMFA"`
	UpdateInfo             bool `json:"updateInfo"`
	UpdateLimits           bool `json:"updateLimits"`
	UpdatePlacement        bool `json:"updatePlacement"`
	UpdateStatus           bool `json:"updateStatus"`
	UpdateValueAttribution bool `json:"updateValueAttribution"`
	View                   bool `json:"view"`
}

// ProjectFlags are the feature flags related to projects.
type ProjectFlags struct {
	Create                 bool `json:"create"`
	Delete                 bool `json:"delete"`
	History                bool `json:"history"`
	List                   bool `json:"list"`
	UpdateInfo             bool `json:"updateInfo"`
	UpdateLimits           bool `json:"updateLimits"`
	UpdatePlacement        bool `json:"updatePlacement"`
	UpdateValueAttribution bool `json:"updateValueAttribution"`
	View                   bool `json:"view"`
	MemberList             bool `json:"memberList"`
	MemberAdd              bool `json:"memberAdd"`
	MemberRemove           bool `json:"memberRemove"`
}

// BucketFlags are the feature flags related to buckets.
type BucketFlags struct {
	Create                 bool `json:"create"`
	Delete                 bool `json:"delete"`
	History                bool `json:"history"`
	List                   bool `json:"list"`
	UpdateInfo             bool `json:"updateInfo"`
	UpdatePlacement        bool `json:"updatePlacement"`
	UpdateValueAttribution bool `json:"updateValueAttribution"`
	View                   bool `json:"view"`
}

// GetSettings returns the service settings.
func (s *Service) GetSettings(ctx context.Context) (*Settings, api.HTTPError) {
	return &Settings{
		Admin: SettingsAdmin{
			Features: FeatureFlags{
				Account: AccountFlags{
					Create:                 false,
					Delete:                 false,
					History:                false,
					List:                   false,
					Projects:               true,
					Search:                 true,
					Suspend:                false,
					Unsuspend:              false,
					ResetMFA:               false,
					UpdateInfo:             false,
					UpdateLimits:           false,
					UpdatePlacement:        false,
					UpdateStatus:           false,
					UpdateValueAttribution: false,
					View:                   true,
				},
				Project: ProjectFlags{
					Create:                 false,
					Delete:                 false,
					History:                false,
					List:                   false,
					UpdateInfo:             false,
					UpdateLimits:           true,
					UpdatePlacement:        false,
					UpdateValueAttribution: false,
					View:                   true,
					MemberList:             false,
					MemberAdd:              false,
					MemberRemove:           false,
				},
				Bucket: BucketFlags{
					Create:                 false,
					Delete:                 false,
					History:                false,
					List:                   false,
					UpdateInfo:             false,
					UpdatePlacement:        false,
					UpdateValueAttribution: false,
					View:                   false,
				},
				Dashboard:       false,
				Operator:        false,
				SignOut:         false,
				SwitchSatellite: false,
			},
		},
	}, api.HTTPError{}
}
