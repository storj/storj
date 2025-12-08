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
	MaxAddFundsAmount                 int                       `help:"maximum amount (in cents) allowed to be added to an account balance." default:"250000"`
	MinAddFundsAmount                 int                       `help:"minimum amount (in cents) allowed to be added to an account balance." default:"1000"`
	UpgradePayUpfrontAmount           int                       `help:"amount (in cents) required to upgrade to a paid tier, use 0 to disable" default:"500"`
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
	AbbreviatedDeleteProjectEnabled   bool                      `help:"whether the abbreviated delete project flow is enabled" default:"false"`
	SelfServeAccountDeleteEnabled     bool                      `help:"whether self-serve account delete flow is enabled" default:"false"`
	AbbreviatedDeleteAccountEnabled   bool                      `help:"whether the abbreviated self-serve delete account flow is enabled" default:"false"`
	UseNewRestKeysTable               bool                      `help:"whether to use the new rest keys table" default:"false"`
	NewDetailedUsageReportEnabled     bool                      `help:"whether to use the new detailed usage report" default:"false"`
	PricingPackagesEnabled            bool                      `help:"whether to allow purchasing pricing packages" default:"true"`
	SkuEnabled                        bool                      `help:"whether we should use SKUs for product usages" default:"false" hidden:"true"`
	UserFeedbackEnabled               bool                      `help:"whether user feedback is enabled" default:"false"`
	AuditableAPIKeyProjects           []string                  `help:"list of public project IDs for which auditable API keys are enabled" default:"[]" hidden:"true"`
	ValidAnnouncementNames            []string                  `help:"list of valid announcement names that can be used in the UI" default:"[]"`
	ComputeUiEnabled                  bool                      `help:"whether the compute UI is enabled" default:"false"`
	ShowNewPricingTiers               bool                      `help:"whether to show new pricing tiers in the UI" default:"false"`
	EntitlementsEnabled               bool                      `help:"whether entitlements are enabled" default:"false" hidden:"true"`
	NewPricingStartDate               string                    `help:"the date (YYYY-MM-DD) when new pricing tiers will be enabled" default:"2025-11-01"`
	ProductPriceSummaries             []string                  `help:"the pricing summaries gotten from configured products" default:"" hidden:"true"`
	MemberAccountsEnabled             bool                      `help:"whether member accounts are enabled" default:"false"`
	CollectBillingInfoOnOnboarding    bool                      `help:"whether to collect billing information during onboarding" default:"false"`

	LegacyPlacements                          []string                 `help:"list of placement IDs that are considered legacy placements" default:""`
	LegacyPlacementProductMappingForMigration PlacementProductMappings `help:"mapping of legacy placement IDs to product IDs for migration" default:""`

	PartnerUI  PartnerUIConfig        `help:"partner-specific UI configuration in YAML format or file path"`
	WhiteLabel TenantWhiteLabelConfig `help:"tenant-specific white label configuration in YAML format or file path"`

	ManagedEncryption SatelliteManagedEncryptionConfig
	RestAPIKeys       RestAPIKeysConfig
	Placement         PlacementsConfig
	UsageLimits       UsageLimitsConfig
	Captcha           CaptchaConfig
	Session           SessionConfig
	AccountFreeze     AccountFreezeConfig
	Announcement      AnnouncementConfig

	SupportURL string `help:"url link to general request page" hidden:"true"`
	LoginURL   string `help:"url link to the satellite UI login" hidden:"true"`
}

// AnnouncementConfig contains configurations for announcements shown in the UI.
type AnnouncementConfig struct {
	Enabled bool   `help:"indicates whether announcement should be shown in the UI" default:"false" json:"enabled"`
	Name    string `help:"name of the announcement" default:"" json:"name"`
	Title   string `help:"title of the announcement" default:"" json:"title"`
	Body    string `help:"body of the announcement" default:"" json:"body"`
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
	SelfServeEnabled                  bool                              `help:"whether self-serve placement selection feature is enabled" default:"false"`
	SelfServeDetails                  PlacementDetails                  `help:"human-readable details for placements allowed for self serve placement. See satellite/console/README.md for more details."`
	AllowedPlacementIdsForNewProjects AllowedPlacementIDsForNewProjects `help:"list of placement IDs that are allowed for new projects, e.g.[0, 10]" default:"[]"`
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

// AllowedPlacementIDsForNewProjects represents a list of placement IDs that are allowed for new projects.
type AllowedPlacementIDsForNewProjects []storj.PlacementConstraint

// Ensure that AllowedPlacementIDsForNewProjects implements pflag.Value.
var _ pflag.Value = (*AllowedPlacementIDsForNewProjects)(nil)

// Type implements pflag.Value.
func (*AllowedPlacementIDsForNewProjects) Type() string {
	return "console.AllowedPlacementIDsForNewProjects"
}

// String implements pflag.Value.
func (ap *AllowedPlacementIDsForNewProjects) String() string {
	if ap == nil || len(*ap) == 0 {
		return ""
	}

	placements, err := json.Marshal(ap)
	if err != nil {
		return ""
	}

	return string(placements)
}

// Set implements pflag.Value.
func (ap *AllowedPlacementIDsForNewProjects) Set(s string) error {
	if s == "" {
		return nil
	}

	var placements []storj.PlacementConstraint
	err := json.Unmarshal([]byte(s), &placements)
	if err != nil {
		return err
	}
	*ap = placements

	return nil
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
	ShortName   string `json:"shortName" yaml:"short-name"`
	Title       string `json:"title" yaml:"title"`
	Description string `json:"description" yaml:"description"`
	// WaitlistURL is only parsed from configuration and not sent to the front-end.
	WaitlistURL string `json:"waitlist_url,omitempty" yaml:"wait-list-url,omitempty"`
	// Pending indicates whether the placement has a waitlist - to be sent to the front-end.
	Pending    bool   `json:"pending" yaml:"-"`
	LucideIcon string `json:"lucideIcon,omitempty" yaml:"lucide-icon,omitempty"`
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

// PlacementProductMappings represents a mapping between placement IDs and product IDs.
type PlacementProductMappings struct {
	mappings map[storj.PlacementConstraint]int32
}

// Ensure that PlacementProductMappings implements pflag.Value.
var _ pflag.Value = (*PlacementProductMappings)(nil)

// Type returns the type of the pflag.Value.
func (*PlacementProductMappings) Type() string { return "entitlements.PlacementProductMappings" }

// String returns a string representation of the PlacementProductMappings.
func (ppm *PlacementProductMappings) String() string {
	if ppm == nil || len(ppm.mappings) == 0 {
		return ""
	}

	data, err := json.Marshal(ppm.mappings)
	if err != nil {
		return ""
	}

	return string(data)
}

// Set parses and sets the PlacementProductMappings from a string.
func (ppm *PlacementProductMappings) Set(value string) error {
	if value == "" {
		return nil
	}

	value = strings.TrimSpace(value)

	mappings := make(map[storj.PlacementConstraint]int32)
	if err := json.Unmarshal([]byte(value), &mappings); err != nil {
		return errs.New("failed to parse PlacementProductMappings: %w", err)
	}

	ppm.mappings = mappings

	return nil
}

// UIConfig contains UI configuration for different parts of the UI.
type UIConfig struct {
	Billing     map[string]any `yaml:"billing,omitempty"`
	Onboarding  map[string]any `yaml:"onboarding,omitempty"`
	Upgrade     map[string]any `yaml:"upgrade,omitempty"`
	PricingPlan map[string]any `yaml:"pricing-plan,omitempty"`
	Signup      map[string]any `yaml:"signup,omitempty"`
}

// PartnerUIConfig contains partner-specific UI configuration.
type PartnerUIConfig struct {
	Value map[string]UIConfig
}

var _ pflag.Value = (*PartnerUIConfig)(nil)

// Set parses a YAML file or string into PartnerUIConfig.
func (p *PartnerUIConfig) Set(s string) error {
	if s == "" {
		return nil
	}

	s = strings.TrimSpace(s)
	strBytes := []byte(s)
	var cfg map[string]UIConfig
	switch {
	case strings.HasSuffix(s, ".yaml"):
		// YAML file path
		data, err := os.ReadFile(s)
		if err != nil {
			return errs.New("Couldn't read partner UI config file from %s: %v", s, err)
		}

		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			return errs.New("failed to parse partner UI config YAML file: %v", err)
		}
	default:
		// YAML string
		err := yaml.Unmarshal(strBytes, &cfg)
		if err != nil {
			return errs.New("failed to parse config YAML: %v", err)
		}
	}

	*p = PartnerUIConfig{Value: cfg}
	return nil
}

// String returns the YAML representation of PartnerUIConfig.
func (p *PartnerUIConfig) String() string {
	if p == nil {
		return ""
	}

	bytes, err := yaml.Marshal(p.Value)
	if err != nil {
		return ""
	}

	str := string(bytes)
	if str == "{}\n" {
		return ""
	}

	return string(bytes)
}

// Type returns the type of the pflag.Value.
func (p *PartnerUIConfig) Type() string {
	return "console.PartnerUIConfig"
}

// TenantWhiteLabelConfig contains white-label UI configuration; a mapping of tenant IDs to their configurations.
type TenantWhiteLabelConfig struct {
	Value map[string]WhiteLabelConfig
	// HostNameIDLookup is a reverse mapping of host names to tenant IDs,
	// added for efficient lookup based on incoming request host names.
	HostNameIDLookup map[string]string
}

// WhiteLabelConfig contains white-label configuration for a tenant.
type WhiteLabelConfig struct {
	TenantID      string            `yaml:"-"`
	HostName      string            `yaml:"host-name,omitempty"`
	Name          string            `yaml:"name,omitempty"`
	LogoURLs      map[string]string `yaml:"logo-urls,omitempty"`
	FaviconURLs   map[string]string `yaml:"favicon-urls,omitempty"`
	Colors        map[string]string `yaml:"colors,omitempty"`
	SupportURL    string            `yaml:"support-url,omitempty"`
	DocsURL       string            `yaml:"docs-url,omitempty"`
	HomepageURL   string            `yaml:"homepage-url,omitempty"`
	GetInTouchURL string            `yaml:"get-in-touch-url,omitempty"`
}

var _ pflag.Value = (*TenantWhiteLabelConfig)(nil)

// Set parses a YAML file or string into TenantWhiteLabelConfig.
func (t *TenantWhiteLabelConfig) Set(s string) error {
	if s == "" {
		return nil
	}

	s = strings.TrimSpace(s)
	strBytes := []byte(s)
	var cfg map[string]WhiteLabelConfig
	switch {
	case strings.HasSuffix(s, ".yaml"):
		// YAML file path
		data, err := os.ReadFile(s)
		if err != nil {
			return errs.New("Couldn't read white label config file from %s: %v", s, err)
		}

		err = yaml.Unmarshal(data, &cfg)
		if err != nil {
			return errs.New("failed to parse white label config YAML file: %v", err)
		}
	default:
		// YAML string
		err := yaml.Unmarshal(strBytes, &cfg)
		if err != nil {
			return errs.New("failed to parse config YAML: %v", err)
		}
	}

	hostNameIDLookup := make(map[string]string)
	for id, config := range cfg {
		if config.HostName == "" {
			return errs.New("white label config for tenant ID %s is missing host name", id)
		}
		hostNameIDLookup[config.HostName] = id
		config.TenantID = id
		cfg[id] = config
	}

	*t = TenantWhiteLabelConfig{Value: cfg, HostNameIDLookup: hostNameIDLookup}
	return nil
}

// String returns the YAML representation of TenantWhiteLabelConfig.
func (t *TenantWhiteLabelConfig) String() string {
	if t == nil {
		return ""
	}

	bytes, err := yaml.Marshal(t.Value)
	if err != nil {
		return ""
	}

	str := string(bytes)
	if str == "{}\n" {
		return ""
	}

	return string(bytes)
}

// Type returns the type of the pflag.Value.
func (t *TenantWhiteLabelConfig) Type() string {
	return "console.TenantWhiteLabelConfig"
}
