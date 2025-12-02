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
	Create              bool `json:"create"`
	CreateRestKey       bool `json:"createRestKey"`
	Delete              bool `json:"delete"`
	MarkPendingDeletion bool `json:"markPendingDeletion"`
	History             bool `json:"history"`
	List                bool `json:"list"`
	Projects            bool `json:"projects"`
	Search              bool `json:"search"`
	Suspend             bool `json:"suspend"`
	Unsuspend           bool `json:"unsuspend"`
	DisableMFA          bool `json:"disableMFA"`
	UpdateLimits        bool `json:"updateLimits"`
	UpdatePlacement     bool `json:"updatePlacement"`
	UpdateStatus        bool `json:"updateStatus"`
	UpdateEmail         bool `json:"updateEmail"`
	UpdateKind          bool `json:"updateKind"`
	UpdateName          bool `json:"updateName"`
	UpdateUserAgent     bool `json:"updateUserAgent"`
	View                bool `json:"view"`
}

// ProjectFlags are the feature flags related to projects.
type ProjectFlags struct {
	Create                 bool `json:"create"`
	Delete                 bool `json:"delete"`
	MarkPendingDeletion    bool `json:"markPendingDeletion"`
	History                bool `json:"history"`
	List                   bool `json:"list"`
	UpdateInfo             bool `json:"updateInfo"`
	UpdateLimits           bool `json:"updateLimits"`
	UpdatePlacement        bool `json:"updatePlacement"`
	UpdateValueAttribution bool `json:"updateValueAttribution"`
	SetEntitlements        bool `json:"setEntitlements"`
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

// GetSettings returns the service settings based on the caller's permissions.
func (s *Service) GetSettings(_ context.Context, authInfo *AuthInfo) (*Settings, api.HTTPError) {
	var settings Settings
	for _, g := range authInfo.Groups {
		// account permission features
		if s.authorizer.HasPermissions(g, PermAccountView) {
			settings.Admin.Features.Account.View = true
			settings.Admin.Features.Account.Search = true
			settings.Admin.Features.Account.Projects = true
		}
		if s.authorizer.HasPermissions(g, PermAccountSuspendTemporary, PermAccountSuspendPermanently) {
			settings.Admin.Features.Account.Suspend = true
		}
		if s.authorizer.HasPermissions(g, PermAccountReActivateTemporary, PermAccountReActivatePermanently) {
			settings.Admin.Features.Account.Unsuspend = true
		}
		if s.authorizer.HasPermissions(g, PermAccountChangeName) {
			settings.Admin.Features.Account.UpdateName = true
		}
		if s.authorizer.HasPermissions(g, PermAccountChangeKind) {
			settings.Admin.Features.Account.UpdateKind = true
		}
		if s.authorizer.HasPermissions(g, PermAccountSetUserAgent) {
			settings.Admin.Features.Account.UpdateUserAgent = true
		}
		if s.authorizer.HasPermissions(g, PermAccountChangeStatus) {
			settings.Admin.Features.Account.UpdateStatus = true
		}
		if s.authorizer.HasPermissions(g, PermAccountChangeLimits) {
			settings.Admin.Features.Account.UpdateLimits = true
		}
		if s.authorizer.HasPermissions(g, PermAccountChangeEmail) {
			settings.Admin.Features.Account.UpdateEmail = true
		}
		if s.authorizer.HasPermissions(g, PermAccountDeleteNoData) {
			settings.Admin.Features.Account.Delete = true
		}
		if s.adminConfig.PendingDeleteUserCleanupEnabled && s.authorizer.HasPermissions(g, PermAccountMarkPendingDeletion, PermAccountDeleteWithData) {
			settings.Admin.Features.Account.MarkPendingDeletion = true
		}
		if s.authorizer.HasPermissions(g, PermAccountDisableMFA) {
			settings.Admin.Features.Account.DisableMFA = true
		}
		if s.authorizer.HasPermissions(g, PermAccountCreateRestKey) {
			settings.Admin.Features.Account.CreateRestKey = true
		}
		if s.authorizer.HasPermissions(g, PermAccountView, PermViewChangeHistory) {
			settings.Admin.Features.Account.History = true
		}

		// project permission features
		if s.authorizer.HasPermissions(g, PermProjectView) {
			settings.Admin.Features.Project.View = true
		}
		if s.authorizer.HasPermissions(g, PermProjectSetLimits) {
			settings.Admin.Features.Project.UpdateLimits = true
		}
		if s.authorizer.HasPermissions(g, PermProjectUpdate) {
			settings.Admin.Features.Project.UpdateInfo = true
		}
		if s.authorizer.HasPermissions(g, PermProjectSetUserAgent) {
			settings.Admin.Features.Project.UpdateValueAttribution = true
		}
		if s.authorizer.HasPermissions(g, PermProjectSetDataPlacement) {
			settings.Admin.Features.Project.UpdatePlacement = true
		}
		if s.authorizer.HasPermissions(g, PermProjectSetEntitlements) {
			settings.Admin.Features.Project.SetEntitlements = true
		}
		if s.authorizer.HasPermissions(g, PermProjectDeleteNoData) {
			settings.Admin.Features.Project.Delete = true
		}
		if s.adminConfig.PendingDeleteProjectCleanupEnabled && s.authorizer.HasPermissions(g, PermProjectMarkPendingDeletion) {
			settings.Admin.Features.Project.MarkPendingDeletion = true
		}
		if s.authorizer.HasPermissions(g, PermProjectView, PermViewChangeHistory) {
			settings.Admin.Features.Project.History = true
		}

		// bucket permission features
		if s.authorizer.HasPermissions(g, PermProjectView, PermBucketView) {
			settings.Admin.Features.Bucket.List = true
		}
		if s.authorizer.HasPermissions(g, PermProjectView, PermBucketView) {
			settings.Admin.Features.Bucket.View = true
		}
		if s.authorizer.HasPermissions(g, PermProjectView, PermBucketSetDataPlacement) {
			settings.Admin.Features.Bucket.UpdatePlacement = true
		}
		if s.authorizer.HasPermissions(g, PermProjectView, PermBucketSetUserAgent) {
			settings.Admin.Features.Bucket.UpdateValueAttribution = true
		}
		if s.authorizer.HasPermissions(g, PermProjectView, PermBucketView, PermViewChangeHistory) {
			settings.Admin.Features.Bucket.History = true
		}
	}

	return &settings, api.HTTPError{}
}
