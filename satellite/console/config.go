// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"gopkg.in/yaml.v3"

	"storj.io/common/storj"
)

// Config keeps track of core console service configuration parameters.
type Config struct {
	PasswordCost                      int                       `help:"password hashing cost (0=automatic)" testDefault:"4" default:"0"`
	OpenRegistrationEnabled           bool                      `help:"enable open registration" default:"false" testDefault:"true"`
	DefaultProjectLimit               int                       `help:"default project limits for users" default:"1" testDefault:"5"`
	AsOfSystemTimeDuration            time.Duration             `help:"default duration for AS OF SYSTEM TIME" devDefault:"-5m" releaseDefault:"-5m" testDefault:"0"`
	LoginAttemptsWithoutPenalty       int                       `help:"number of times user can try to login without penalty" default:"3"`
	FailedLoginPenalty                float64                   `help:"incremental duration of penalty for failed login attempts in minutes" default:"2.0"`
	ProjectInvitationExpiration       time.Duration             `help:"duration that project member invitations are valid for" default:"168h"`
	UnregisteredInviteEmailsEnabled   bool                      `help:"indicates whether invitation emails can be sent to unregistered email addresses" default:"true"`
	UserBalanceForUpgrade             int64                     `help:"amount of base units of US micro dollars needed to upgrade user's tier status" default:"10000000"`
	PlacementEdgeURLOverrides         PlacementEdgeURLOverrides `help:"placement-specific edge service URL overrides in the format {\"placementID\": {\"authService\": \"...\", \"publicLinksharing\": \"...\", \"internalLinksharing\": \"...\"}, \"placementID2\": ...}"`
	BlockExplorerURL                  string                    `help:"url of the transaction block explorer" default:"https://etherscan.io/"`
	ZkSyncBlockExplorerURL            string                    `help:"url of the zkSync transaction block explorer" default:"https://explorer.zksync.io/"`
	ZkSyncContractAddress             string                    `help:"the STORJ zkSync Era contract address" default:"0xA0806DA7835a4E63dB2CE44A2b622eF8b73B5DB5"`
	BillingFeaturesEnabled            bool                      `help:"indicates if billing features should be enabled" default:"true"`
	MaxAddFundsAmount                 int                       `help:"maximum amount (in cents) allowed to be added to an account balance." default:"10000"`
	MinAddFundsAmount                 int                       `help:"minimum amount (in cents) allowed to be added to an account balance." default:"1000"`
	UpgradePayUpfrontAmount           int                       `help:"amount (in cents) required to upgrade to a paid tier, use 0 to disable" default:"0"`
	SignupActivationCodeEnabled       bool                      `help:"indicates whether the whether account activation is done using activation code" default:"true" testDefault:"false" devDefault:"false"`
	FreeTrialDuration                 time.Duration             `help:"duration for which users can access the system free of charge, 0 = unlimited time trial" default:"0"`
	VarPartners                       []string                  `help:"list of partners whose users will not see billing UI." default:""`
	ObjectBrowserKeyNamePrefix        string                    `help:"prefix for object browser API key names" default:".storj-web-file-browser-api-key-"`
	ObjectBrowserKeyLifetime          time.Duration             `help:"duration for which the object browser API key remains valid" default:"72h"`
	MaxNameCharacters                 int                       `help:"defines the maximum number of characters allowed for names, e.g. user first/last names and company names" default:"100"`
	MaxLongFormFieldCharacters        int                       `help:"defines the maximum number of characters allowed for long form fields, e.g. comment type fields" default:"500"`
	BillingInformationTabEnabled      bool                      `help:"indicates if billing information tab should be enabled" default:"false"`
	SatelliteManagedEncryptionEnabled bool                      `help:"indicates whether satellite managed encryption projects can be created." default:"false"`
	EmailChangeFlowEnabled            bool                      `help:"whether change user email flow is enabled" default:"false"`
	DeleteProjectEnabled              bool                      `help:"whether project deletion from satellite UI is enabled" default:"false"`
	SelfServeAccountDeleteEnabled     bool                      `help:"whether self-serve account delete flow is enabled" default:"false"`
	UseNewRestKeysTable               bool                      `help:"whether to use the new rest keys table" default:"false"`
	NewDetailedUsageReportEnabled     bool                      `help:"whether to use the new detailed usage report" default:"false"`
	NewAccountSetupEnabled            bool                      `help:"whether to use new account setup flow" default:"false"`
	ProductBasedInvoicing             bool                      `help:"whether to use product-based invoicing" default:"false" hidden:"true"`
	PricingPackagesEnabled            bool                      `help:"whether to allow purchasing pricing packages" default:"true"`
	SkuEnabled                        bool                      `help:"whether we should use SKUs for product usages" default:"false" hidden:"true"`
	UserFeedbackEnabled               bool                      `help:"whether user feedback is enabled" default:"false"`
	AuditableAPIKeyProjects           []string                  `help:"list of public project IDs for which auditable API keys are enabled" default:"[]" hidden:"true"`

	ManagedEncryption SatelliteManagedEncryptionConfig
	RestAPIKeys       RestAPIKeysConfig
	Placement         PlacementsConfig
	UsageLimits       UsageLimitsConfig
	Captcha           CaptchaConfig
	Session           SessionConfig
	AccountFreeze     AccountFreezeConfig

	SupportURL string `help:"url link to general request page" hidden:"true"`
	LoginURL   string `help:"url link to the satellite UI login" hidden:"true"`
}

// SatelliteManagedEncryptionConfig contains configurations for Satellite Managed Encryption.
type SatelliteManagedEncryptionConfig struct {
	PathEncryptionEnabled bool `help:"indicates whether projects with managed encryption should have path encryption enabled" default:"false"`
}

// RestAPIKeysConfig contains configurations for REST API keys.
type RestAPIKeysConfig struct {
	DefaultExpiration time.Duration `help:"expiration to use if user does not specify an rest key expiration" default:"720h"`
}

// PlacementsConfig contains configurations for self-serve placement logic.
type PlacementsConfig struct {
	SelfServeEnabled bool             `help:"whether self-serve placement selection feature is enabled" default:"false"`
	SelfServeDetails PlacementDetails `help:"human-readable details for placements allowed for self serve placement. See satellite/console/README.md for more details."`
}

// CaptchaConfig contains configurations for login/registration captcha system.
type CaptchaConfig struct {
	FlagBotsEnabled      bool               `help:"indicates if flagging bot accounts is enabled" default:"false" json:"-"`
	ScoreCutoffThreshold float64            `help:"bad captcha score threshold which is used to prevent bot user activity" default:"0.8" json:"-"`
	MinFlagBotDelay      int                `help:"min number of days before flagging a bot account" default:"1" json:"-"`
	MaxFlagBotDelay      int                `help:"max number of days before flagging a bot account" default:"7" json:"-"`
	Login                MultiCaptchaConfig `json:"login"`
	Registration         MultiCaptchaConfig `json:"registration"`
}

// MultiCaptchaConfig contains configurations for Recaptcha and Hcaptcha systems.
type MultiCaptchaConfig struct {
	Recaptcha SingleCaptchaConfig `json:"recaptcha"`
	Hcaptcha  SingleCaptchaConfig `json:"hcaptcha"`
}

// SingleCaptchaConfig contains configurations abstract captcha system.
type SingleCaptchaConfig struct {
	Enabled   bool   `help:"whether or not captcha is enabled" default:"false" json:"enabled"`
	SiteKey   string `help:"captcha site key" json:"siteKey"`
	SecretKey string `help:"captcha secret key" json:"-"`
}

// SessionConfig contains configurations for session management.
type SessionConfig struct {
	InactivityTimerEnabled       bool          `help:"indicates if session can be timed out due inactivity" default:"true"`
	InactivityTimerDuration      int           `help:"inactivity timer delay in seconds" default:"1800"` // 1800s=30m
	InactivityTimerViewerEnabled bool          `help:"indicates whether remaining session time is shown for debugging" default:"false"`
	Duration                     time.Duration `help:"duration a session is valid for (superseded by inactivity timer delay if inactivity timer is enabled)" default:"168h"`
}

// ObjectLockAndVersioningConfig contains configurations for object versioning.
type ObjectLockAndVersioningConfig struct {
	ObjectLockEnabled              bool
	UseBucketLevelObjectVersioning bool
}

// EdgeURLOverrides contains edge service URL overrides.
type EdgeURLOverrides struct {
	AuthService         string `json:"authService,omitempty"`
	PublicLinksharing   string `json:"publicLinksharing,omitempty"`
	InternalLinksharing string `json:"internalLinksharing,omitempty"`
}

// PlacementEdgeURLOverrides represents a mapping between placement IDs and edge service URL overrides.
type PlacementEdgeURLOverrides struct {
	overrideMap map[storj.PlacementConstraint]EdgeURLOverrides
}

// Ensure that PlacementEdgeOverrides implements pflag.Value.
var _ pflag.Value = (*PlacementEdgeURLOverrides)(nil)

// Type implements pflag.Value.
func (PlacementEdgeURLOverrides) Type() string { return "console.PlacementEdgeURLOverrides" }

// String implements pflag.Value.
func (ov *PlacementEdgeURLOverrides) String() string {
	if ov == nil || len(ov.overrideMap) == 0 {
		return ""
	}

	overrides, err := json.Marshal(ov.overrideMap)
	if err != nil {
		return ""
	}

	return string(overrides)
}

// Set implements pflag.Value.
func (ov *PlacementEdgeURLOverrides) Set(s string) error {
	if s == "" {
		return nil
	}

	overrides := make(map[storj.PlacementConstraint]EdgeURLOverrides)
	err := json.Unmarshal([]byte(s), &overrides)
	if err != nil {
		return err
	}
	ov.overrideMap = overrides

	return nil
}

// Get returns the edge service URL overrides for the given placement ID.
func (ov *PlacementEdgeURLOverrides) Get(placement storj.PlacementConstraint) (overrides EdgeURLOverrides, ok bool) {
	if ov == nil {
		return EdgeURLOverrides{}, false
	}
	overrides, ok = ov.overrideMap[placement]
	return overrides, ok
}

// PlacementDetail represents human-readable details of a placement.
type PlacementDetail struct {
	ID          int    `json:"id" yaml:"id"`
	IdName      string `json:"idName" yaml:"id-name"`
	Name        string `json:"name" yaml:"name"`
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description" yaml:"description"`
	// WaitlistURL is only parsed from configuration and not sent to the front-end.
	WaitlistURL string `json:"waitlist_url,omitempty" yaml:"wait-list-url,omitempty"`
	// Pending indicates whether the placement has a waitlist - to be sent to the front-end.
	Pending bool `json:"pending" yaml:"-"`
}

// PlacementDetails represents a mapping between placement IDs and their human-readable details.
type PlacementDetails []PlacementDetail

// Ensure that PlacementDetails implements pflag.Value.
var _ pflag.Value = (*PlacementDetails)(nil)

// Type implements pflag.Value.
func (PlacementDetails) Type() string { return "console.PlacementDetails" }

// String implements pflag.Value.
func (pd *PlacementDetails) String() string {
	if pd == nil || len(*pd) == 0 {
		return ""
	}

	bytes, err := yaml.Marshal(pd)
	if err != nil {
		return ""
	}

	return string(bytes)
}

// SetMap sets the internal mapping between a placement and detail.
func (pd *PlacementDetails) SetMap(overrides map[storj.PlacementConstraint]PlacementDetail) {
	details := make([]PlacementDetail, 0, len(overrides))
	for _, detail := range overrides {
		details = append(details, detail)
	}
	*pd = details
}

// GetMap returns the internal mapping between a placement and detail.
func (pd *PlacementDetails) GetMap() map[storj.PlacementConstraint]PlacementDetail {
	detailMap := make(map[storj.PlacementConstraint]PlacementDetail, len(*pd))
	for _, detail := range *pd {
		detailMap[storj.PlacementConstraint(detail.ID)] = detail
	}
	return detailMap
}

// Set implements pflag.Value.
func (pd *PlacementDetails) Set(s string) error {
	if s == "" {
		return nil
	}

	s = strings.TrimSpace(s)
	strBytes := []byte(s)

	var details PlacementDetails
	switch {
	case strings.HasSuffix(s, ".yaml"):
		// YAML file path
		data, err := os.ReadFile(s)
		if err != nil {
			return errs.New("Couldn't read placement config file from %s: %v", s, err)
		}

		err = yaml.Unmarshal(data, &details)
		if err != nil {
			return errs.New("failed to parse placement config YAML file: %v", err)
		}
	case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"):
		// JSON string
		// This is how this config is defined in production.
		var jsonForm map[storj.PlacementConstraint]PlacementDetail
		err := json.Unmarshal(strBytes, &jsonForm)
		if err != nil {
			return errs.New("failed to parse placement config JSON: %v", err)
		}
		for id, detail := range jsonForm {
			detail.ID = int(id)
			details = append(details, detail)
		}
	default:
		// YAML string
		err := yaml.Unmarshal(strBytes, &details)
		if err != nil {
			return errs.New("failed to parse placement config YAML: %v", err)
		}
	}

	*pd = details

	return nil
}

// Get returns the details for the given placement ID.
func (pd *PlacementDetails) Get(placement storj.PlacementConstraint) (details PlacementDetail, ok bool) {
	if pd == nil {
		return PlacementDetail{}, false
	}
	for _, detail := range *pd {
		if detail.ID == int(placement) {
			return detail, true
		}
	}
	return PlacementDetail{}, false
}
