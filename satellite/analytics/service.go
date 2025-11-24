// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	segment "gopkg.in/segmentio/analytics-go.v3"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

var mon = monkit.Package()

const (
	// SourceTrialExpiringNotice is the trial expiring notice source.
	SourceTrialExpiringNotice = "trial_expiring_notice"
	// SourceTrialExpiredNotice is the trial expired notice source.
	SourceTrialExpiredNotice = "trial_expired_notice"

	eventInviteLinkClicked                = "Invite Link Clicked"
	eventInviteLinkSignup                 = "Invite Link Signup"
	eventAccountCreated                   = "Account Created"
	eventAccountDeleted                   = "Account Deleted"
	eventAccountSetUp                     = "Account Set Up"
	eventSignedIn                         = "Signed In"
	eventProjectCreated                   = "Project Created"
	eventProjectDeleted                   = "Project Deleted"
	eventManagedEncryptionError           = "Managed Encryption Error"
	eventAccessGrantCreated               = "Access Grant Created"
	eventAccountVerified                  = "Account Verified"
	eventGatewayCredentialsCreated        = "Credentials Created"
	eventPassphraseCreated                = "Passphrase Created"
	eventExternalLinkClicked              = "External Link Clicked"
	eventPathSelected                     = "Path Selected"
	eventLinkShared                       = "Link Shared"
	eventObjectUploaded                   = "Object Uploaded"
	eventAPIKeyGenerated                  = "API Key Generated"
	eventCreditCardAdded                  = "Credit Card Added"
	eventUpgradeBannerClicked             = "Upgrade Banner Clicked"
	eventModalAddCard                     = "Credit Card Added In Modal"
	eventModalAddTokens                   = "Storj Token Added In Modal"
	eventSearchBuckets                    = "Search Buckets"
	eventNavigateProjects                 = "Navigate Projects"
	eventManageProjectsClicked            = "Manage Projects Clicked"
	eventCreateNewClicked                 = "Create New Clicked"
	eventViewDocsClicked                  = "View Docs Clicked"
	eventViewForumClicked                 = "View Forum Clicked"
	eventViewSupportClicked               = "View Support Clicked"
	eventCreateAnAccessGrantClicked       = "Create an Access Grant Clicked"
	eventUploadUsingCliClicked            = "Upload Using CLI Clicked"
	eventUploadInWebClicked               = "Upload In Web Clicked"
	eventNewProjectClicked                = "New Project Clicked"
	eventLogoutClicked                    = "Logout Clicked"
	eventProfileUpdated                   = "Profile Updated"
	eventPasswordChanged                  = "Password Changed"
	eventMfaEnabled                       = "MFA Enabled"
	eventBucketCreated                    = "Bucket Created"
	eventBucketDeleted                    = "Bucket Deleted"
	eventProjectLimitError                = "Project Limit Error"
	eventAPIAccessCreated                 = "API Access Created"
	eventUploadFileClicked                = "Upload File Clicked"
	eventUploadFolderClicked              = "Upload Folder Clicked"
	eventStorjTokenAdded                  = "Storj Token Added"
	eventCreateKeysClicked                = "Create Keys Clicked"
	eventDownloadTxtClicked               = "Download txt clicked"
	eventEncryptMyAccessClicked           = "Encrypt My Access Clicked"
	eventCopyToClipboardClicked           = "Copy to Clipboard Clicked"
	eventCreateAccessGrantClicked         = "Create Access Grant Clicked"
	eventCreateS3CredentialsClicked       = "Create S3 Credentials Clicked"
	eventKeysForCLIClicked                = "Create Keys For CLI Clicked"
	eventSeePaymentsClicked               = "See Payments Clicked"
	eventEditPaymentMethodClicked         = "Edit Payment Method Clicked"
	eventUsageDetailedInfoClicked         = "Usage Detailed Info Clicked"
	eventAddNewPaymentMethodClicked       = "Add New Payment Method Clicked"
	eventApplyNewCouponClicked            = "Apply New Coupon Clicked"
	eventCreditCardRemoved                = "Credit Card Removed"
	eventCouponCodeApplied                = "Coupon Code Applied"
	eventInvoiceDownloaded                = "Invoice Downloaded"
	eventCreditCardAddedFromBilling       = "Credit Card Added From Billing"
	eventStorjTokenAddedFromBilling       = "Storj Token Added From Billing"
	eventAddFundsClicked                  = "Add Funds Clicked"
	eventProjectMembersInviteSent         = "Project Members Invite Sent"
	eventProjectMemberAdded               = "Project Member Added"
	eventProjectMemberDeleted             = "Project Member Deleted"
	eventError                            = "UI error occurred"
	eventProjectNameUpdated               = "Project Name Updated"
	eventProjectDescriptionUpdated        = "Project Description Updated"
	eventProjectStorageLimitUpdated       = "Project Storage Limit Updated"
	eventProjectBandwidthLimitUpdated     = "Project Bandwidth Limit Updated"
	eventAccountFrozen                    = "Account Frozen"
	eventAccountUnfrozen                  = "Account Unfrozen"
	eventAccountUnwarned                  = "Account Unwarned"
	eventAccountFreezeWarning             = "Account Freeze Warning"
	eventUnpaidLargeInvoice               = "Large Invoice Unpaid"
	eventUnpaidStorjscanInvoice           = "Storjscan Invoice Unpaid"
	eventPendingDeletionUnpaidInvoice     = "Pending Deletion Invoice Open"
	eventLegalHoldUnpaidInvoice           = "Legal Hold Invoice Open"
	eventExpiredCreditNeedsRemoval        = "Expired Credit Needs Removal"
	eventExpiredCreditRemoved             = "Expired Credit Removed"
	eventProjectInvitationAccepted        = "Project Invitation Accepted"
	eventProjectInvitationDeclined        = "Project Invitation Declined"
	eventGalleryViewClicked               = "Gallery View Clicked"
	eventResendInviteClicked              = "Resend Invite Clicked"
	eventCopyInviteLinkClicked            = "Copy Invite Link Clicked"
	eventRemoveProjectMemberCLicked       = "Remove Member Clicked"
	eventLimitIncreaseRequested           = "Limit Increase Requested"
	eventUserSignUp                       = "User Sign Up"
	eventPersonalInfoSubmitted            = "Personal Info Submitted"
	eventBusinessInfoSubmitted            = "Business Info Submitted"
	eventUseCaseSelected                  = "Use Case Selected"
	eventOnboardingCompleted              = "Onboarding Completed"
	eventOnboardingAbandoned              = "Onboarding Abandoned"
	eventPersonalSelected                 = "Personal Selected"
	eventBusinessSelected                 = "Business Selected"
	eventUserUpgraded                     = "User Upgraded"
	eventUpgradeClicked                   = "Upgrade Clicked"
	eventArrivedFromSource                = "Arrived From Source"
	eventApplicationsSetupClicked         = "Applications Setup Clicked"
	eventApplicationsSetupCompleted       = "Applications Setup Completed"
	eventApplicationsDocsClicked          = "Applications Docs Clicked"
	eventCloudGPUNavigationClicked        = "Cloud GPU Navigation Item Clicked"
	eventCloudGPUSignupClicked            = "Cloud GPU Sign Up Clicked"
	eventJoinCunoFSBetaSubmitted          = "Join CunoFS Beta Form Submitted"
	eventJoinPlacementWaitlistSubmitted   = "Join Placement Waitlist Form Submitted"
	eventObjectMountConsultationSubmitted = "Object Mount Consultation Submitted"
	eventAdmitAudit                       = "Admin Audit Event"
	// EventUserFeedbackSubmitted is an event for user feedback submission.
	// Exported to be reused in other packages.
	EventUserFeedbackSubmitted = "User Feedback Submitted"

	// Generic account freeze event types.
	eventAccountFreeze   = "Account Freeze"
	eventAccountUnfreeze = "Account Unfreeze"
)

var (
	// Error is the default error class the analytics package.
	Error = errs.Class("analytics service")
)

// Config is a configuration struct for analytics Service.
type Config struct {
	SegmentWriteKey string `help:"segment write key" default:""`
	Enabled         bool   `help:"enable analytics reporting" default:"false"`
	HubSpot         HubSpotConfig
	Plausible       plausibleConfig
}

// FreezeTracker is an interface for account freeze event tracking methods.
type FreezeTracker interface {
	// TrackAccountFrozen sends an account frozen event to Segment.
	TrackAccountFrozen(userID uuid.UUID, email string, hubspotObjectID *string)

	// TrackAccountUnfrozen sends an account unfrozen event to Segment.
	TrackAccountUnfrozen(userID uuid.UUID, email string, hubspotObjectID *string)

	// TrackAccountUnwarned sends an account unwarned event to Segment.
	TrackAccountUnwarned(userID uuid.UUID, email string, hubspotObjectID *string)

	// TrackAccountFreezeWarning sends an account freeze warning event to Segment.
	TrackAccountFreezeWarning(userID uuid.UUID, email string, hubspotObjectID *string)

	// TrackLargeUnpaidInvoice sends an event to Segment indicating that a user has not paid a large invoice.
	TrackLargeUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string)

	// TrackViolationFrozenUnpaidInvoice sends an event to Segment indicating that a user has not paid an invoice
	// and has been frozen due to violating ToS.
	TrackViolationFrozenUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string)

	// TrackStorjscanUnpaidInvoice sends an event to Segment indicating that a user has not paid an invoice, but has storjscan transaction history.
	TrackStorjscanUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string)

	// TrackGenericFreeze sends a generic account freeze event to Segment with the freeze type specified.
	// The adminInitiated parameter specifies whether the freeze was initiated by an admin action or automatically.
	TrackGenericFreeze(userID uuid.UUID, email, freezeType string, adminInitiated bool, hubspotObjectID *string)

	// TrackGenericUnfreeze sends a generic account unfreeze event to Segment with the freeze type specified.
	// The adminInitiated parameter specifies whether the unfreeze was initiated by an admin action or automatically.
	TrackGenericUnfreeze(userID uuid.UUID, email, freezeType string, adminInitiated bool, hubspotObjectID *string)
}

// LimitRequestInfo holds data needed to request limit increase.
type LimitRequestInfo struct {
	ProjectName  string
	LimitType    string
	CurrentLimit string
	DesiredLimit string
}

// Service for sending analytics.
//
// architecture: Service
type Service struct {
	log                      *zap.Logger
	config                   Config
	satelliteName            string
	satelliteExternalAddress string
	clientEvents             map[string]bool
	sources                  map[string]interface{}

	segment   segment.Client
	hubspot   *HubSpotEvents
	plausible *plausibleService
}

// NewService creates new service for creating sending analytics.
func NewService(log *zap.Logger, config Config, satelliteName, satelliteExternalAddress string) *Service {
	service := &Service{
		log:                      log,
		config:                   config,
		satelliteName:            satelliteName,
		satelliteExternalAddress: satelliteExternalAddress,
		clientEvents:             make(map[string]bool),
		sources:                  make(map[string]interface{}),
		hubspot:                  NewHubSpotEvents(log.Named("hubspotclient"), config.HubSpot, satelliteName),
		plausible:                newPlausibleService(log.Named("plausibleservice"), config.Plausible),
	}
	if config.Enabled {
		service.segment = segment.New(config.SegmentWriteKey)
	}
	for _, name := range []string{eventGatewayCredentialsCreated, eventPassphraseCreated, eventExternalLinkClicked,
		eventPathSelected, eventLinkShared, eventObjectUploaded, eventAPIKeyGenerated, eventUpgradeBannerClicked,
		eventModalAddCard, eventModalAddTokens, eventSearchBuckets, eventNavigateProjects, eventManageProjectsClicked,
		eventCreateNewClicked, eventViewDocsClicked, eventViewForumClicked, eventViewSupportClicked, eventCreateAnAccessGrantClicked,
		eventUploadUsingCliClicked, eventUploadInWebClicked, eventNewProjectClicked, eventLogoutClicked, eventProfileUpdated,
		eventPasswordChanged, eventMfaEnabled, eventBucketCreated, eventBucketDeleted, eventAccessGrantCreated, eventAPIAccessCreated,
		eventUploadFileClicked, eventUploadFolderClicked, eventCreateKeysClicked, eventDownloadTxtClicked, eventEncryptMyAccessClicked,
		eventCopyToClipboardClicked, eventCreateAccessGrantClicked, eventCreateS3CredentialsClicked, eventKeysForCLIClicked,
		eventSeePaymentsClicked, eventEditPaymentMethodClicked, eventUsageDetailedInfoClicked, eventAddNewPaymentMethodClicked,
		eventApplyNewCouponClicked, eventCreditCardRemoved, eventCouponCodeApplied, eventInvoiceDownloaded, eventCreditCardAddedFromBilling,
		eventStorjTokenAddedFromBilling, eventAddFundsClicked, eventProjectMembersInviteSent, eventError, eventProjectNameUpdated, eventProjectDescriptionUpdated,
		eventProjectStorageLimitUpdated, eventProjectBandwidthLimitUpdated, eventProjectInvitationAccepted, eventProjectInvitationDeclined,
		eventGalleryViewClicked, eventResendInviteClicked, eventRemoveProjectMemberCLicked, eventCopyInviteLinkClicked, eventUserSignUp,
		eventPersonalInfoSubmitted, eventBusinessInfoSubmitted, eventUseCaseSelected, eventOnboardingCompleted, eventOnboardingAbandoned,
		eventPersonalSelected, eventBusinessSelected, eventUserUpgraded, eventUpgradeClicked, eventArrivedFromSource, eventApplicationsDocsClicked,
		eventApplicationsSetupClicked, eventApplicationsSetupCompleted, eventCloudGPUNavigationClicked, eventCloudGPUSignupClicked,
		eventJoinCunoFSBetaSubmitted, eventJoinPlacementWaitlistSubmitted, eventObjectMountConsultationSubmitted, EventUserFeedbackSubmitted} {
		service.clientEvents[name] = true
	}

	service.sources[SourceTrialExpiredNotice] = struct{}{}
	service.sources[SourceTrialExpiringNotice] = struct{}{}

	return service
}

// Run runs the service and use the context in new requests.
func (service *Service) Run(ctx context.Context) error {
	if !service.config.Enabled {
		return nil
	}
	return service.hubspot.Run(ctx)
}

// Close closes the Segment client.
func (service *Service) Close() error {
	if !service.config.Enabled {
		return nil
	}
	return service.segment.Close()
}

// UserType is a type for distinguishing personal vs. professional users.
type UserType string

const (
	// Professional defines a "professional" user type.
	Professional UserType = "Professional"
	// Personal defines a "personal" user type.
	Personal UserType = "Personal"
)

// TrackCreateUserFields contains input data for tracking a create user event.
type TrackCreateUserFields struct {
	ID               uuid.UUID
	TenantID         *string
	AnonymousID      string
	FullName         string
	Email            string
	Type             UserType
	EmployeeCount    string
	CompanyName      string
	StorageNeeds     string
	JobTitle         string
	HaveSalesContact bool
	OriginHeader     string
	Referrer         string
	HubspotUTK       string
	UserAgent        string
	SignupCaptcha    *float64
}

// TrackJoinCunoFSBetaFields contains input data for tracking a join CunoFS beta event.
type TrackJoinCunoFSBetaFields struct {
	Email                       string `json:"email"`
	CompanyName                 string `json:"companyName"`
	FirstName                   string `json:"firstName"`
	LastName                    string `json:"lastName"`
	IndustryUseCase             string `json:"industryUseCase"`
	OtherIndustryUseCase        string `json:"otherIndustryUseCase"`
	OperatingSystem             string `json:"operatingSystem"`
	TeamSize                    string `json:"teamSize"`
	CurrentStorageUsage         string `json:"currentStorageUsage"`
	InfraType                   string `json:"infraType"`
	CurrentStorageBackends      string `json:"currentStorageBackends"`
	OtherStorageBackend         string `json:"otherStorageBackend"`
	CurrentStorageMountSolution string `json:"currentStorageMountSolution"`
	OtherStorageMountSolution   string `json:"otherStorageMountSolution"`
	DesiredFeatures             string `json:"desiredFeatures"`
	CurrentPainPoints           string `json:"currentPainPoints"`
	SpecificTasks               string `json:"specificTasks"`
}

// TrackJoinPlacementWaitlistFields contains input data for join placement waitlist event.
type TrackJoinPlacementWaitlistFields struct {
	Email        string                    `json:"email"`
	StorageNeeds string                    `json:"storageNeeds"`
	WaitlistURL  string                    `json:"-"`
	Placement    storj.PlacementConstraint `json:"placement"`
}

// TrackObjectMountConsultationFields contains input data for tracking an object mount consultation event.
type TrackObjectMountConsultationFields struct {
	Email                  string `json:"email"`
	CompanyName            string `json:"companyName"`
	FirstName              string `json:"firstName"`
	LastName               string `json:"lastName"`
	JobTitle               string `json:"jobTitle"`
	PhoneNumber            string `json:"phoneNumber"`
	IndustryUseCase        string `json:"industryUseCase"`
	CompanySize            string `json:"companySize"`
	CurrentStorageSolution string `json:"currentStorageSolution"`
	KeyChallenges          string `json:"keyChallenges"`
	SpecificInterests      string `json:"specificInterests"`
	StorageNeeds           string `json:"storageNeeds"`
	ImplementationTimeline string `json:"implementationTimeline"`
	AdditionalInformation  string `json:"additionalInformation"`
}

// TrackOnboardingInfoFields contains input data entered after first login.
type TrackOnboardingInfoFields struct {
	ID                     uuid.UUID
	TenantID               *string
	HubspotObjectID        *string
	FullName               string
	Email                  string
	Type                   UserType
	EmployeeCount          string
	CompanyName            string
	StorageNeeds           string
	JobTitle               string
	StorageUseCase         string
	OtherUseCase           string
	FunctionalArea         string
	HaveSalesContact       bool
	InterestedInPartnering bool
}

// UserFeedbackFormData is the data submitted by the user feedback form.
type UserFeedbackFormData struct {
	Type         string `json:"type"`
	Message      string `json:"message"`
	ReproSteps   string `json:"reproSteps"`
	AllowContact bool   `json:"allowContact"`
}

func (service *Service) enqueueMessage(message segment.Message) {
	err := service.segment.Enqueue(message)
	if err != nil {
		service.log.Error("Error enqueueing message", zap.Error(err))
	}
}

// TrackCreateUser sends an "Account Created" event to Segment and Hubspot.
func (service *Service) TrackCreateUser(fields TrackCreateUserFields) {
	if !service.config.Enabled {
		return
	}

	fullName := fields.FullName
	names := strings.SplitN(fullName, " ", 2)

	var firstName string
	var lastName string

	if len(names) > 1 {
		firstName = names[0]
		lastName = names[1]
	} else {
		firstName = fullName
	}

	traits := segment.NewTraits()
	traits.SetFirstName(firstName)
	traits.SetLastName(lastName)
	traits.SetEmail(fields.Email)
	traits.Set("origin_header", fields.OriginHeader)
	traits.Set("signup_referrer", fields.Referrer)
	traits.Set("account_created", true)
	if fields.Type == Professional {
		traits.Set("have_sales_contact", fields.HaveSalesContact)
	}
	if len(fields.UserAgent) > 0 {
		traits.Set("signup_partner", fields.UserAgent)
	}

	service.enqueueMessage(segment.Identify{
		UserId:      fields.ID.String(),
		AnonymousId: fields.AnonymousID,
		Traits:      traits,
	})

	props := service.newPropertiesWithOpts(nil, fields.TenantID)
	props.Set("email", fields.Email)
	props.Set("name", fields.FullName)
	props.Set("account_type", fields.Type)
	props.Set("origin_header", fields.OriginHeader)
	props.Set("signup_referrer", fields.Referrer)
	props.Set("account_created", true)
	if fields.SignupCaptcha != nil {
		props.Set("signup_captcha", &fields.SignupCaptcha)
	}

	if fields.Type == Professional {
		props.Set("company_size", fields.EmployeeCount)
		props.Set("company_name", fields.CompanyName)
		props.Set("job_title", fields.JobTitle)
		props.Set("storage_needs", fields.StorageNeeds)
	}

	service.enqueueMessage(segment.Track{
		UserId:      fields.ID.String(),
		AnonymousId: fields.AnonymousID,
		Event:       eventAccountCreated,
		Properties:  props,
	})

	service.hubspot.EnqueueCreateUserMinimal(fields)
}

// TrackDeleteUser sends an "Account Deleted" event to Segment.
func (service *Service) TrackDeleteUser(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccountDeleted,
		Properties: props,
	})
}

// JoinCunoFSBeta sends a join cunoFS beta form to hubspot.
func (service *Service) JoinCunoFSBeta(fields TrackJoinCunoFSBetaFields) {
	if !service.config.Enabled {
		return
	}
	service.hubspot.EnqueueJoinCunoFSBeta(fields)
}

// JoinPlacementWaitlist sends a join placement waitlist form to hubspot.
func (service *Service) JoinPlacementWaitlist(fields TrackJoinPlacementWaitlistFields) {
	if !service.config.Enabled {
		return
	}
	service.hubspot.EnqueueJoinPlacementWaitlist(fields)
}

// RequestObjectMountConsultation sends an object mount consultation form to hubspot.
func (service *Service) RequestObjectMountConsultation(fields TrackObjectMountConsultationFields) {
	if !service.config.Enabled {
		return
	}
	service.hubspot.EnqueueObjectMountConsultation(fields)
}

// ChangeContactEmail changes contact's email address.
func (service *Service) ChangeContactEmail(userID uuid.UUID, oldEmail, newEmail string) {
	if !service.config.Enabled {
		return
	}

	traits := segment.NewTraits()
	traits.SetEmail(newEmail)

	service.enqueueMessage(segment.Identify{
		UserId: userID.String(),
		Traits: traits,
	})

	service.hubspot.EnqueueUserChangeEmail(oldEmail, newEmail)
}

// TrackUserOnboardingInfo sends onboarding info to Hubspot.
func (service *Service) TrackUserOnboardingInfo(fields TrackOnboardingInfoFields) {
	if !service.config.Enabled {
		return
	}

	fullName := fields.FullName
	names := strings.SplitN(fullName, " ", 2)

	var firstName string
	var lastName string

	if len(names) > 1 {
		firstName = names[0]
		lastName = names[1]
	} else {
		firstName = fullName
	}

	traits := segment.NewTraits()
	traits.SetFirstName(firstName)
	traits.SetLastName(lastName)
	traits.SetEmail(fields.Email)
	if fields.Type == Professional {
		traits.Set("have_sales_contact", fields.HaveSalesContact)
		traits.Set("interested_in_partnering", fields.InterestedInPartnering)
	}

	service.enqueueMessage(segment.Identify{
		UserId: fields.ID.String(),
		Traits: traits,
	})

	props := service.newPropertiesWithOpts(fields.HubspotObjectID, fields.TenantID)
	props.Set("email", fields.Email)
	props.Set("name", fields.FullName)
	props.Set("account_type", fields.Type)
	props.Set("storage_use", fields.StorageUseCase)
	props.Set("other_use_case", fields.OtherUseCase)

	if fields.Type == Professional {
		props.Set("company_size", fields.EmployeeCount)
		props.Set("company_name", fields.CompanyName)
		props.Set("job_title", fields.JobTitle)
		props.Set("storage_needs", fields.StorageNeeds)
		props.Set("functional_area", fields.FunctionalArea)
	}

	service.enqueueMessage(segment.Track{
		UserId:     fields.ID.String(),
		Event:      eventAccountSetUp,
		Properties: props,
	})

	service.hubspot.EnqueueUserOnboardingInfo(fields)
}

// TrackSignedIn sends an "Signed In" event to Segment.
func (service *Service) TrackSignedIn(userID uuid.UUID, email, anonymousID string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	traits := segment.NewTraits()
	traits.SetEmail(email)

	service.enqueueMessage(segment.Identify{
		UserId:      userID.String(),
		AnonymousId: anonymousID,
		Traits:      traits,
	})

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventSignedIn,
		Properties: props,
	})
}

// TrackProjectCreated sends an "Project Created" event to Segment.
func (service *Service) TrackProjectCreated(userID uuid.UUID, email string, projectID uuid.UUID, currentProjectCount int, managedPassphrase bool, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("project_count", currentProjectCount)
	props.Set("project_id", projectID.String())
	props.Set("email", email)

	encManagedBy := "user"
	if managedPassphrase {
		encManagedBy = "satellite"
	}
	props.Set("encryption_managed_by", encManagedBy)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventProjectCreated,
		Properties: props,
	})
}

// TrackProjectDeleted sends an "Project Deleted" event to Segment.
func (service *Service) TrackProjectDeleted(userID uuid.UUID, email string, publicProjectID uuid.UUID, currentMonthUsage string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("project_id", publicProjectID.String())
	props.Set("email", email)
	props.Set("current_usage_price", currentMonthUsage)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventProjectDeleted,
		Properties: props,
	})
}

// TrackManagedEncryptionError sends an "Managed Encryption Error" event to Segment.
func (service *Service) TrackManagedEncryptionError(userID uuid.UUID, email string, projectID uuid.UUID, reason string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("project_id", projectID.String())
	props.Set("email", email)
	props.Set("reason", reason)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventManagedEncryptionError,
		Properties: props,
	})
}

// TrackAccountFrozen sends an account frozen event to Segment.
func (service *Service) TrackAccountFrozen(userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccountFrozen,
		Properties: props,
	})
}

// TrackRequestLimitIncrease sends a limit increase request to Segment.
func (service *Service) TrackRequestLimitIncrease(userID uuid.UUID, email string, info LimitRequestInfo, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)
	if info.ProjectName != "" {
		props.Set("project", info.ProjectName)
	}
	props.Set("type", info.LimitType)
	props.Set("currentLimit", info.CurrentLimit)
	props.Set("desiredLimit", info.DesiredLimit)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventLimitIncreaseRequested,
		Properties: props,
	})
}

// TrackAccountUnfrozen sends an account unfrozen event to Segment.
func (service *Service) TrackAccountUnfrozen(userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccountUnfrozen,
		Properties: props,
	})
}

// TrackGenericFreeze sends a generic account freeze event to Segment with the freeze type specified.
func (service *Service) TrackGenericFreeze(userID uuid.UUID, email, freezeType string, adminInitiated bool, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)
	props.Set("freeze_type", freezeType)
	props.Set("admin_initiated", adminInitiated)

	service.log.Info("user frozen", zap.String("email", email), zap.String("user_id", userID.String()), zap.String("freezeType", freezeType), zap.Bool("adminInitiated", adminInitiated))

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccountFreeze,
		Properties: props,
	})
}

// TrackGenericUnfreeze sends a generic account unfreeze event to Segment with the freeze type specified.
func (service *Service) TrackGenericUnfreeze(userID uuid.UUID, email, freezeType string, adminInitiated bool, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)
	props.Set("freeze_type", freezeType)
	props.Set("admin_initiated", adminInitiated)

	service.log.Info("user unfrozen", zap.String("email", email), zap.String("user_id", userID.String()), zap.String("freezeType", freezeType), zap.Bool("adminInitiated", adminInitiated))

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccountUnfreeze,
		Properties: props,
	})
}

// TrackAccountUnwarned sends an account unwarned event to Segment.
func (service *Service) TrackAccountUnwarned(userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccountUnwarned,
		Properties: props,
	})
}

// TrackAccountFreezeWarning sends an account freeze warning event to Segment.
func (service *Service) TrackAccountFreezeWarning(userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccountFreezeWarning,
		Properties: props,
	})
}

// TrackLargeUnpaidInvoice sends an event to Segment indicating that a user has not paid a large invoice.
func (service *Service) TrackLargeUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)
	props.Set("invoice", invID)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventUnpaidLargeInvoice,
		Properties: props,
	})
}

// TrackViolationFrozenUnpaidInvoice sends an event to Segment indicating that a violation frozen user has not paid an invoice.
func (service *Service) TrackViolationFrozenUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)
	props.Set("invoice", invID)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventPendingDeletionUnpaidInvoice,
		Properties: props,
	})
}

// TrackLegalHoldUnpaidInvoice sends an event to Segment indicating that a user has not paid an invoice
// but is in legal hold.
func (service *Service) TrackLegalHoldUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)
	props.Set("invoice", invID)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventLegalHoldUnpaidInvoice,
		Properties: props,
	})
}

// TrackStorjscanUnpaidInvoice sends an event to Segment indicating that a user has not paid an invoice, but has storjscan transaction history.
func (service *Service) TrackStorjscanUnpaidInvoice(invID string, userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)
	props.Set("invoice", invID)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventUnpaidStorjscanInvoice,
		Properties: props,
	})
}

// TrackAccessGrantCreated sends an "Access Grant Created" event to Segment.
func (service *Service) TrackAccessGrantCreated(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccessGrantCreated,
		Properties: props,
	})
}

// TrackAccountVerified sends an "Account Verified" event to Segment.
func (service *Service) TrackAccountVerified(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	traits := segment.NewTraits()
	traits.SetEmail(email)

	service.enqueueMessage(segment.Identify{
		UserId: userID.String(),
		Traits: traits,
	})

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAccountVerified,
		Properties: props,
	})
}

// TrackEvent sends an arbitrary event associated with user ID to Segment.
// It is used for tracking occurrences of client-side events.
func (service *Service) TrackEvent(eventName string, userID uuid.UUID, email string, customProps map[string]string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	// do not track if the event name is an invalid client-side event
	if !service.clientEvents[eventName] {
		service.log.Error("Invalid client-triggered event", zap.String("eventName", eventName))
		return
	}

	if v, ok := customProps["source"]; ok {
		if _, ok = service.sources[v]; !ok {
			service.log.Error("Event source is not in allowed list", zap.String("eventName", eventName), zap.String("source", v))
			return
		}
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	for key, value := range customProps {
		props.Set(key, value)
	}

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventName,
		Properties: props,
	})
}

// TrackErrorEvent sends an arbitrary error event associated with user ID to Segment.
// It is used for tracking occurrences of client-side errors.
func (service *Service) TrackErrorEvent(userID uuid.UUID, email, source, requestID string, statusCode int, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)
	props.Set("source", source)

	if requestID != "" {
		props.Set("request_id", requestID)
	}
	if statusCode != 0 {
		props.Set("status_code", statusCode)
	}

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventError,
		Properties: props,
	})
}

// TrackLinkEvent sends an arbitrary event and link associated with user ID to Segment.
// It is used for tracking occurrences of client-side events.
func (service *Service) TrackLinkEvent(eventName string, userID uuid.UUID, email, link string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	// do not track if the event name is an invalid client-side event
	if !service.clientEvents[eventName] {
		service.log.Error("Invalid client-triggered event", zap.String("eventName", eventName))
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("link", link)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventName,
		Properties: props,
	})
}

// TrackCreditCardAdded sends an "Credit Card Added" event to Segment.
func (service *Service) TrackCreditCardAdded(userID uuid.UUID, email string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventCreditCardAdded,
		Properties: props,
	})
}

// PageVisitEvent sends a page visit event associated with user ID to Segment.
// It is used for tracking occurrences of client-side events.
func (service *Service) PageVisitEvent(pageName string, userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)
	props.Set("path", pageName)
	props.Set("user_id", userID.String())

	service.enqueueMessage(segment.Page{
		UserId:     userID.String(),
		Name:       "Page Requested",
		Properties: props,
	})
}

// PageViewEvent sends a page view event to plausible.
func (service *Service) PageViewEvent(ctx context.Context, pv PageViewBody) error {
	if !service.config.Enabled {
		return nil
	}

	return service.plausible.pageViewEvent(ctx, pv)
}

// TrackProjectLimitError sends an "Project Limit Error" event to Segment.
func (service *Service) TrackProjectLimitError(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventProjectLimitError,
		Properties: props,
	})
}

// TrackStorjTokenAdded sends an "Storj Token Added" event to Segment.
func (service *Service) TrackStorjTokenAdded(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventStorjTokenAdded,
		Properties: props,
	})
}

// TrackProjectMemberAddition sends an "Project Member Added" event to Segment.
func (service *Service) TrackProjectMemberAddition(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventProjectMemberAdded,
		Properties: props,
	})
}

// TrackProjectMemberDeletion sends an "Project Member Deleted" event to Segment.
func (service *Service) TrackProjectMemberDeletion(userID uuid.UUID, email string, hubspotObjectID, tenantID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, tenantID)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventProjectMemberDeleted,
		Properties: props,
	})
}

// TrackExpiredCreditNeedsRemoval sends an "Expired Credit Needs Removal" event to Segment.
func (service *Service) TrackExpiredCreditNeedsRemoval(userID uuid.UUID, customerID, packagePlan string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("customer ID", customerID)
	props.Set("package plan", packagePlan)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventExpiredCreditNeedsRemoval,
		Properties: props,
	})
}

// TrackExpiredCreditRemoved sends an "Expired Credit Removed" event to Segment.
func (service *Service) TrackExpiredCreditRemoved(userID uuid.UUID, customerID, packagePlan string, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("customer ID", customerID)
	props.Set("package plan", packagePlan)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventExpiredCreditRemoved,
		Properties: props,
	})
}

// TrackInviteLinkSignup sends an "Invite Link Signup" event to Segment.
func (service *Service) TrackInviteLinkSignup(inviter, invitee string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(nil, nil)
	props.Set("inviter", inviter)
	props.Set("invitee", invitee)

	service.enqueueMessage(segment.Track{
		Event:      eventInviteLinkSignup,
		Properties: props,
	})
}

// TrackInviteLinkClicked sends an "Invite Link Clicked" event to Segment.
func (service *Service) TrackInviteLinkClicked(inviter, invitee string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(nil, nil)
	props.Set("inviter", inviter)
	props.Set("invitee", invitee)

	service.enqueueMessage(segment.Track{
		Event:      eventInviteLinkClicked,
		Properties: props,
	})
}

// TrackUserUpgraded sends a "User Upgraded" event to Segment.
func (service *Service) TrackUserUpgraded(userID uuid.UUID, email string, expiration *time.Time, hubspotObjectID *string) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(hubspotObjectID, nil)
	props.Set("email", email)

	now := time.Now()

	// NOTE: if this runs before legacy free tier migration, old free tier will
	// be considered unlimited.
	if expiration == nil {
		props.Set("trial status", "unlimited")
	} else {
		if now.After(*expiration) {
			props.Set("trial status", "expired")
			props.Set("days since expiration", math.Floor(now.Sub(*expiration).Hours()/24))
		} else {
			props.Set("trial status", "active")
			props.Set("days until expiration", math.Floor(expiration.Sub(now).Hours()/24))
		}
	}

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventUserUpgraded,
		Properties: props,
	})
}

// TrackAdminAuditEvent sends an admin audit event to Segment with structured properties.
func (service *Service) TrackAdminAuditEvent(userID uuid.UUID, customProps map[string]interface{}) {
	if !service.config.Enabled {
		return
	}

	props := service.newPropertiesWithOpts(nil, nil)
	for key, value := range customProps {
		props.Set(key, value)
	}

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventAdmitAudit,
		Properties: props,
	})
}

// ValidateAccountObjectCreatedRequestSignature validates the signature of the AccountObjectCreatedRequest.
func (service *Service) ValidateAccountObjectCreatedRequestSignature(
	request AccountObjectCreatedRequest,
	signatureHeader, timestampHeader string,
) error {
	if !service.config.Enabled {
		return nil
	}

	timestampInt, err := strconv.ParseInt(timestampHeader, 10, 64)
	if err != nil {
		return Error.New("invalid request timestamp")
	}

	if time.Since(time.UnixMilli(timestampInt)) > service.config.HubSpot.WebhookRequestLifetime {
		return Error.New("webhook request is too old")
	}

	jsonBytes, err := json.Marshal(request)
	if err != nil {
		return Error.Wrap(err)
	}

	link, err := url.JoinPath(service.satelliteExternalAddress, service.config.HubSpot.AccountObjectCreatedWebhookEndpoint)
	if err != nil {
		return Error.Wrap(err)
	}

	rawString := http.MethodPost + link + string(jsonBytes) + timestampHeader

	h := hmac.New(sha256.New, []byte(service.config.HubSpot.ClientSecret))
	if _, err = h.Write([]byte(rawString)); err != nil {
		return Error.Wrap(err)
	}
	hashedString := base64.StdEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(hashedString), []byte(signatureHeader)) {
		return Error.New("signature does not match: request is invalid")
	}

	return nil
}

// GetAccessToken retrieves the access token from HubSpot.
func (service *Service) GetAccessToken(ctx context.Context) (token string, err error) {
	if !service.config.Enabled {
		return "", Error.New("analytics service is not enabled")
	}

	return service.hubspot.getAccessToken(ctx)
}

// TestSetSatelliteExternalAddress sets the satellite external address for testing purposes.
func (service *Service) TestSetSatelliteExternalAddress(address string) {
	service.satelliteExternalAddress = address
}

func (service *Service) newPropertiesWithOpts(hubspotObjectID, tenantID *string) segment.Properties {
	props := segment.NewProperties()
	props.Set("satellite", service.satelliteName)
	if hubspotObjectID != nil {
		props.Set("hubspot_object_id", *hubspotObjectID)
	}
	if tenantID != nil {
		props.Set("tenant_id", *tenantID)
	}

	return props
}
