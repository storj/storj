// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"encoding/json"
	"time"

	"storj.io/common/memory"
	"storj.io/storj/satellite/console"
)

// FrontendConfig holds the configuration for the satellite frontend.
type FrontendConfig struct {
	ExternalAddress                   string                `json:"externalAddress"`
	SatelliteName                     string                `json:"satelliteName"`
	SatelliteNodeURL                  string                `json:"satelliteNodeURL"`
	StripePublicKey                   string                `json:"stripePublicKey"`
	PartneredSatellites               []PartneredSatellite  `json:"partneredSatellites"`
	DefaultProjectLimit               int                   `json:"defaultProjectLimit"`
	GeneralRequestURL                 string                `json:"generalRequestURL"`
	ProjectLimitsIncreaseRequestURL   string                `json:"projectLimitsIncreaseRequestURL"`
	GatewayCredentialsRequestURL      string                `json:"gatewayCredentialsRequestURL"`
	IsBetaSatellite                   bool                  `json:"isBetaSatellite"`
	BetaSatelliteFeedbackURL          string                `json:"betaSatelliteFeedbackURL"`
	BetaSatelliteSupportURL           string                `json:"betaSatelliteSupportURL"`
	DocumentationURL                  string                `json:"documentationURL"`
	CouponCodeBillingUIEnabled        bool                  `json:"couponCodeBillingUIEnabled"`
	CouponCodeSignupUIEnabled         bool                  `json:"couponCodeSignupUIEnabled"`
	FileBrowserFlowDisabled           bool                  `json:"fileBrowserFlowDisabled"`
	LinksharingURL                    string                `json:"linksharingURL"`
	PublicLinksharingURL              string                `json:"publicLinksharingURL"`
	PathwayOverviewEnabled            bool                  `json:"pathwayOverviewEnabled"`
	Captcha                           console.CaptchaConfig `json:"captcha"`
	LimitsAreaEnabled                 bool                  `json:"limitsAreaEnabled"`
	DefaultPaidStorageLimit           memory.Size           `json:"defaultPaidStorageLimit"`
	DefaultPaidBandwidthLimit         memory.Size           `json:"defaultPaidBandwidthLimit"`
	InactivityTimerEnabled            bool                  `json:"inactivityTimerEnabled"`
	InactivityTimerDuration           int                   `json:"inactivityTimerDuration"`
	InactivityTimerViewerEnabled      bool                  `json:"inactivityTimerViewerEnabled"`
	OptionalSignupSuccessURL          string                `json:"optionalSignupSuccessURL"`
	HomepageURL                       string                `json:"homepageURL"`
	NativeTokenPaymentsEnabled        bool                  `json:"nativeTokenPaymentsEnabled"`
	PasswordMinimumLength             int                   `json:"passwordMinimumLength"`
	PasswordMaximumLength             int                   `json:"passwordMaximumLength"`
	ABTestingEnabled                  bool                  `json:"abTestingEnabled"`
	PricingPackagesEnabled            bool                  `json:"pricingPackagesEnabled"`
	GalleryViewEnabled                bool                  `json:"galleryViewEnabled"`
	NeededTransactionConfirmations    int                   `json:"neededTransactionConfirmations"`
	BillingFeaturesEnabled            bool                  `json:"billingFeaturesEnabled"`
	StripePaymentElementEnabled       bool                  `json:"stripePaymentElementEnabled"`
	UnregisteredInviteEmailsEnabled   bool                  `json:"unregisteredInviteEmailsEnabled"`
	UserBalanceForUpgrade             int64                 `json:"userBalanceForUpgrade"`
	LimitIncreaseRequestEnabled       bool                  `json:"limitIncreaseRequestEnabled"`
	SignupActivationCodeEnabled       bool                  `json:"signupActivationCodeEnabled"`
	AllowedUsageReportDateRange       time.Duration         `json:"allowedUsageReportDateRange"`
	EnableRegionTag                   bool                  `json:"enableRegionTag"`
	EmissionImpactViewEnabled         bool                  `json:"emissionImpactViewEnabled"`
	DaysBeforeTrialEndNotification    int                   `json:"daysBeforeTrialEndNotification"`
	AnalyticsEnabled                  bool                  `json:"analyticsEnabled"`
	ObjectBrowserKeyNamePrefix        string                `json:"objectBrowserKeyNamePrefix"`
	ObjectBrowserKeyLifetime          time.Duration         `json:"objectBrowserKeyLifetime"`
	MaxNameCharacters                 int                   `json:"maxNameCharacters"`
	BillingInformationTabEnabled      bool                  `json:"billingInformationTabEnabled"`
	SatelliteManagedEncryptionEnabled bool                  `json:"satelliteManagedEncryptionEnabled"`
	EmailChangeFlowEnabled            bool                  `json:"emailChangeFlowEnabled"`
	SelfServeAccountDeleteEnabled     bool                  `json:"selfServeAccountDeleteEnabled"`
	NoLimitsUiEnabled                 bool                  `json:"noLimitsUiEnabled"`
	AltObjBrowserPagingEnabled        bool                  `json:"altObjBrowserPagingEnabled"`
}

// Satellites is a configuration value that contains a list of satellite names and addresses.
// Format should be [{"name": "","address": ""],{"name": "","address": ""},...] in valid JSON format.
//
// Can be used as a flag.
type Satellites []PartneredSatellite

// PartneredSatellite contains the name and web address of a satellite.
type PartneredSatellite struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

// Type implements pflag.Value.
func (Satellites) Type() string { return "consoleweb.Satellites" }

// String is required for pflag.Value.
func (sl *Satellites) String() string {
	satellites, err := json.Marshal(*sl)
	if err != nil {
		return ""
	}

	return string(satellites)
}

// Set does validation on the configured JSON.
func (sl *Satellites) Set(s string) (err error) {
	satellites := make([]PartneredSatellite, 3)

	err = json.Unmarshal([]byte(s), &satellites)
	if err != nil {
		return err
	}

	*sl = satellites
	return
}
