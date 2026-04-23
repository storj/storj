// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"

	"storj.io/storj/private/api"
	"storj.io/storj/satellite/console"
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
	Admin   SettingsAdmin   `json:"admin"`
	Console SettingsConsole `json:"console"`
}

// SettingsConsole are the settings of the console service that are exposed in this service.
type SettingsConsole struct {
	ExternalAddress string   `json:"externalAddress"`
	TenantIDList    []string `json:"tenantIDList"`
	PartnerList     []string `json:"partnerList"`
}

// SettingsAdmin are the settings of this service and the server that exposes it.
type SettingsAdmin struct {
	Features FeatureFlags    `json:"features"`
	Branding *BrandingConfig `json:"branding"`
}

// BrandingConfig contains visual branding settings for the admin UI.
type BrandingConfig struct {
	Name        string            `json:"name"`
	LogoURLs    map[string]string `json:"logoUrls"`
	FaviconURLs map[string]string `json:"faviconUrls"`
	Colors      map[string]string `json:"colors"`
}

// FeatureFlags indicates what Admin service features are enabled or disabled. The features are
// usually disabled when they are not fully implemented.
type FeatureFlags struct {
	Account         AccountFlags `json:"account"`
	Project         ProjectFlags `json:"project"`
	Bucket          BucketFlags  `json:"bucket"`
	Access          AccessFlags  `json:"access"`
	Dashboard       bool         `json:"dashboard"`
	Operator        bool         `json:"operator"` // This is the information about the logged operator
	SignOut         bool         `json:"signOut"`
	SwitchSatellite bool         `json:"switchSatellite"`
}

// AccountFlags are the feature flags related to user's accounts.
type AccountFlags struct {
	Create              bool `json:"create"`
	CreateRestKey       bool `json:"createRestKey"`
	CreateRegToken      bool `json:"createRegToken"`
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
	UpdateUpgradeTime   bool `json:"updateUpgradeTime"`
	UpdateTenantID      bool `json:"updateTenantID"`
	ViewLicenses        bool `json:"viewLicenses"`
	ChangeLicenses      bool `json:"changeLicenses"`
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

// AccessFlags are the feature flags related to accesses.
type AccessFlags struct {
	Inspect bool `json:"inspect"`
	Revoke  bool `json:"revoke"`
}

// GetSettings returns the service settings based on the caller's permissions.
func (s *Service) GetSettings(_ context.Context, authInfo *AuthInfo) (*Settings, api.HTTPError) {
	settings := Settings{
		Console: SettingsConsole{
			ExternalAddress: s.consoleConfig.ExternalAddress,
			TenantIDList:    s.consoleConfig.TenantIDList,
			PartnerList:     s.consoleConfig.PartnerAdminEmailMapping.GetAllPartners(),
		},
	}

	// account permission features
	if s.authorizer.HasPermissions(authInfo, PermAccountView) {
		settings.Admin.Features.Account.View = true
		settings.Admin.Features.Account.Search = true
		settings.Admin.Features.Account.Projects = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountSuspend) {
		settings.Admin.Features.Account.Suspend = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountReActivate) {
		settings.Admin.Features.Account.Unsuspend = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountChangeName) {
		settings.Admin.Features.Account.UpdateName = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountChangeKind) {
		settings.Admin.Features.Account.UpdateKind = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountSetUserAgent) {
		settings.Admin.Features.Account.UpdateUserAgent = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountChangeStatus) {
		settings.Admin.Features.Account.UpdateStatus = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountChangeLimits) {
		settings.Admin.Features.Account.UpdateLimits = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountChangeEmail) {
		settings.Admin.Features.Account.UpdateEmail = true
	}
	if len(s.consoleConfig.TenantIDList) > 0 && s.authorizer.HasPermissions(authInfo, PermAccountUpdateTenantID) {
		settings.Admin.Features.Account.UpdateTenantID = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountDeleteNoData) {
		settings.Admin.Features.Account.Delete = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountMarkPendingDeletion, PermAccountDeleteWithData) {
		settings.Admin.Features.Account.MarkPendingDeletion = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountDisableMFA) {
		settings.Admin.Features.Account.DisableMFA = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountCreateRestKey) {
		settings.Admin.Features.Account.CreateRestKey = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountCreateRegToken) {
		settings.Admin.Features.Account.CreateRegToken = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountView, PermViewChangeHistory) {
		settings.Admin.Features.Account.History = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountSetDataPlacement) {
		settings.Admin.Features.Account.UpdatePlacement = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountChangeUpgradeTime) {
		settings.Admin.Features.Account.UpdateUpgradeTime = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountViewLicenses) {
		settings.Admin.Features.Account.ViewLicenses = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccountChangeLicenses) {
		settings.Admin.Features.Account.ChangeLicenses = true
	}

	// project permission features
	if s.authorizer.HasPermissions(authInfo, PermProjectView) {
		settings.Admin.Features.Project.View = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectSetLimits) {
		settings.Admin.Features.Project.UpdateLimits = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectUpdate) {
		settings.Admin.Features.Project.UpdateInfo = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectSetUserAgent) {
		settings.Admin.Features.Project.UpdateValueAttribution = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectSetDataPlacement) {
		settings.Admin.Features.Project.UpdatePlacement = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectSetEntitlements) {
		settings.Admin.Features.Project.SetEntitlements = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectDeleteNoData) {
		settings.Admin.Features.Project.Delete = true
	}
	if s.adminConfig.PendingDeleteProjectCleanupEnabled && s.authorizer.HasPermissions(authInfo, PermProjectMarkPendingDeletion) {
		settings.Admin.Features.Project.MarkPendingDeletion = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectView, PermViewChangeHistory) {
		settings.Admin.Features.Project.History = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectMembersView) {
		settings.Admin.Features.Project.MemberList = true
	}

	// bucket permission features
	if s.authorizer.HasPermissions(authInfo, PermProjectView, PermBucketView) {
		settings.Admin.Features.Bucket.List = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectView, PermBucketView) {
		settings.Admin.Features.Bucket.View = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectView, PermBucketSetDataPlacement) {
		settings.Admin.Features.Bucket.UpdatePlacement = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectView, PermBucketSetUserAgent) {
		settings.Admin.Features.Bucket.UpdateValueAttribution = true
	}
	if s.authorizer.HasPermissions(authInfo, PermProjectView, PermBucketView, PermViewChangeHistory) {
		settings.Admin.Features.Bucket.History = true
	}

	// access permission features
	if s.authorizer.HasPermissions(authInfo, PermAccessInspect) {
		settings.Admin.Features.Access.Inspect = true
	}
	if s.authorizer.HasPermissions(authInfo, PermAccessRevoke) {
		settings.Admin.Features.Access.Revoke = true
	}

	if s.adminConfig.HideFreezeActions {
		settings.Admin.Features.Account.Suspend = false
		settings.Admin.Features.Account.Unsuspend = false
	}

	if s.tenantID != nil {
		settings.Admin.Features.Account.ViewLicenses = false
		settings.Admin.Features.Account.ChangeLicenses = false
		settings.Admin.Features.Account.UpdateTenantID = false
		settings.Admin.Features.Bucket.History = false
	}

	settings.Admin.Features.Operator = s.adminConfig.OIDC.Enabled
	settings.Admin.Features.SignOut = s.adminConfig.OIDC.Enabled

	if s.consoleConfig.SingleWhiteLabel.Enabled() {
		wl := s.consoleConfig.SingleWhiteLabel.ToWhiteLabelConfig()
		settings.Admin.Branding = brandingFromWhiteLabelConfig(wl)
	}

	return &settings, api.HTTPError{}
}

// brandingFromWhiteLabelConfig converts a console.WhiteLabelConfig to BrandingConfig.
func brandingFromWhiteLabelConfig(wl console.WhiteLabelConfig) *BrandingConfig {
	return &BrandingConfig{
		Name:        wl.Name,
		LogoURLs:    wl.LogoURLs,
		FaviconURLs: wl.FaviconURLs,
		Colors:      wl.Colors,
	}
}
