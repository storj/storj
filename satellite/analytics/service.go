// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"context"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	segment "gopkg.in/segmentio/analytics-go.v3"

	"storj.io/common/uuid"
)

const (
	eventAccountCreated               = "Account Created"
	eventSignedIn                     = "Signed In"
	eventProjectCreated               = "Project Created"
	eventAccessGrantCreated           = "Access Grant Created"
	eventAccountVerified              = "Account Verified"
	eventGatewayCredentialsCreated    = "Credentials Created"
	eventPassphraseCreated            = "Passphrase Created"
	eventExternalLinkClicked          = "External Link Clicked"
	eventPathSelected                 = "Path Selected"
	eventLinkShared                   = "Link Shared"
	eventObjectUploaded               = "Object Uploaded"
	eventAPIKeyGenerated              = "API Key Generated"
	eventCreditCardAdded              = "Credit Card Added"
	eventUpgradeBannerClicked         = "Upgrade Banner Clicked"
	eventModalAddCard                 = "Credit Card Added In Modal"
	eventModalAddTokens               = "Storj Token Added In Modal"
	eventSearchBuckets                = "Search Buckets"
	eventNavigateProjects             = "Navigate Projects"
	eventManageProjectsClicked        = "Manage Projects Clicked"
	eventCreateNewClicked             = "Create New Clicked"
	eventViewDocsClicked              = "View Docs Clicked"
	eventViewForumClicked             = "View Forum Clicked"
	eventViewSupportClicked           = "View Support Clicked"
	eventCreateAnAccessGrantClicked   = "Create an Access Grant Clicked"
	eventUploadUsingCliClicked        = "Upload Using CLI Clicked"
	eventUploadInWebClicked           = "Upload In Web Clicked"
	eventNewProjectClicked            = "New Project Clicked"
	eventLogoutClicked                = "Logout Clicked"
	eventProfileUpdated               = "Profile Updated"
	eventPasswordChanged              = "Password Changed"
	eventMfaEnabled                   = "MFA Enabled"
	eventBucketCreated                = "Bucket Created"
	eventBucketDeleted                = "Bucket Deleted"
	eventProjectLimitError            = "Project Limit Error"
	eventAPIAccessCreated             = "API Access Created"
	eventUploadFileClicked            = "Upload File Clicked"
	eventUploadFolderClicked          = "Upload Folder Clicked"
	eventStorjTokenAdded              = "Storj Token Added"
	eventCreateKeysClicked            = "Create Keys Clicked"
	eventDownloadTxtClicked           = "Download txt clicked"
	eventEncryptMyAccessClicked       = "Encrypt My Access Clicked"
	eventCopyToClipboardClicked       = "Copy to Clipboard Clicked"
	eventCreateAccessGrantClicked     = "Create Access Grant Clicked"
	eventCreateS3CredentialsClicked   = "Create S3 Credentials Clicked"
	eventKeysForCLIClicked            = "Create Keys For CLI Clicked"
	eventSeePaymentsClicked           = "See Payments Clicked"
	eventEditPaymentMethodClicked     = "Edit Payment Method Clicked"
	eventUsageDetailedInfoClicked     = "Usage Detailed Info Clicked"
	eventAddNewPaymentMethodClicked   = "Add New Payment Method Clicked"
	eventApplyNewCouponClicked        = "Apply New Coupon Clicked"
	eventCreditCardRemoved            = "Credit Card Removed"
	eventCouponCodeApplied            = "Coupon Code Applied"
	eventInvoiceDownloaded            = "Invoice Downloaded"
	eventCreditCardAddedFromBilling   = "Credit Card Added From Billing"
	eventStorjTokenAddedFromBilling   = "Storj Token Added From Billing"
	eventAddFundsClicked              = "Add Funds Clicked"
	eventProjectMembersInviteSent     = "Project Members Invite Sent"
	eventProjectMemberAdded           = "Project Member Added"
	eventProjectMemberDeleted         = "Project Member Deleted"
	eventError                        = "UI error occurred"
	eventProjectNameUpdated           = "Project Name Updated"
	eventProjectDescriptionUpdated    = "Project Description Updated"
	eventProjectStorageLimitUpdated   = "Project Storage Limit Updated"
	eventProjectBandwidthLimitUpdated = "Project Bandwidth Limit Updated"
	eventAccountFrozen                = "Account Frozen"
	eventAccountUnfrozen              = "Account Unfrozen"
	eventAccountUnwarned              = "Account Unwarned"
	eventAccountFreezeWarning         = "Account Freeze Warning"
	eventUnpaidLargeInvoice           = "Large Invoice Unpaid"
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
}

// Service for sending analytics.
//
// architecture: Service
type Service struct {
	log           *zap.Logger
	config        Config
	satelliteName string
	clientEvents  map[string]bool

	segment segment.Client
	hubspot *HubSpotEvents
}

// NewService creates new service for creating sending analytics.
func NewService(log *zap.Logger, config Config, satelliteName string) *Service {
	service := &Service{
		log:           log,
		config:        config,
		satelliteName: satelliteName,
		clientEvents:  make(map[string]bool),
		hubspot:       NewHubSpotEvents(log.Named("hubspotclient"), config.HubSpot, satelliteName),
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
		eventProjectStorageLimitUpdated, eventProjectBandwidthLimitUpdated} {
		service.clientEvents[name] = true
	}

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
}

func (service *Service) enqueueMessage(message segment.Message) {
	err := service.segment.Enqueue(message)
	if err != nil {
		service.log.Error("Error enqueueing message", zap.Error(err))
	}
}

// TrackCreateUser sends an "Account Created" event to Segment.
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
	traits.Set("lifecyclestage", "other")
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

	props := segment.NewProperties()
	props.Set("email", fields.Email)
	props.Set("name", fields.FullName)
	props.Set("satellite_selected", service.satelliteName)
	props.Set("account_type", fields.Type)
	props.Set("origin_header", fields.OriginHeader)
	props.Set("signup_referrer", fields.Referrer)
	props.Set("account_created", true)

	if fields.Type == Professional {
		props.Set("company_size", fields.EmployeeCount)
		props.Set("company_name", fields.CompanyName)
		props.Set("job_title", fields.JobTitle)
		props.Set("storage_needs", fields.StorageNeeds)
	}

	service.enqueueMessage(segment.Track{
		UserId:      fields.ID.String(),
		AnonymousId: fields.AnonymousID,
		Event:       service.satelliteName + " " + eventAccountCreated,
		Properties:  props,
	})

	service.hubspot.EnqueueCreateUser(fields)
}

// TrackSignedIn sends an "Signed In" event to Segment.
func (service *Service) TrackSignedIn(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	traits := segment.NewTraits()
	traits.SetEmail(email)

	service.enqueueMessage(segment.Identify{
		UserId: userID.String(),
		Traits: traits,
	})

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventSignedIn,
		Properties: props,
	})

	service.hubspot.EnqueueEvent(email, service.satelliteName+"_"+eventSignedIn, map[string]interface{}{
		"userid": userID.String(),
	})
}

// TrackProjectCreated sends an "Project Created" event to Segment.
func (service *Service) TrackProjectCreated(userID uuid.UUID, email string, projectID uuid.UUID, currentProjectCount int) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("project_count", currentProjectCount)
	props.Set("project_id", projectID.String())
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventProjectCreated,
		Properties: props,
	})

	service.hubspot.EnqueueEvent(email, service.satelliteName+"_"+eventProjectCreated, map[string]interface{}{
		"userid":        userID.String(),
		"project_count": currentProjectCount,
		"project_id":    projectID.String(),
	})
}

// TrackAccountFrozen sends an account frozen event to Segment.
func (service *Service) TrackAccountFrozen(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventAccountFrozen,
		Properties: props,
	})
}

// TrackAccountUnfrozen sends an account unfrozen event to Segment.
func (service *Service) TrackAccountUnfrozen(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventAccountUnfrozen,
		Properties: props,
	})
}

// TrackAccountUnwarned sends an account unwarned event to Segment.
func (service *Service) TrackAccountUnwarned(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventAccountUnwarned,
		Properties: props,
	})
}

// TrackAccountFreezeWarning sends an account freeze warning event to Segment.
func (service *Service) TrackAccountFreezeWarning(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventAccountFreezeWarning,
		Properties: props,
	})
}

// TrackLargeUnpaidInvoice sends an event to Segment indicating that a user has not paid a large invoice.
func (service *Service) TrackLargeUnpaidInvoice(invID string, userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)
	props.Set("invoice", invID)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventUnpaidLargeInvoice,
		Properties: props,
	})
}

// TrackAccessGrantCreated sends an "Access Grant Created" event to Segment.
func (service *Service) TrackAccessGrantCreated(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventAccessGrantCreated,
		Properties: props,
	})

	service.hubspot.EnqueueEvent(email, service.satelliteName+"_"+eventAccessGrantCreated, map[string]interface{}{
		"userid": userID.String(),
	})
}

// TrackAccountVerified sends an "Account Verified" event to Segment.
func (service *Service) TrackAccountVerified(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	traits := segment.NewTraits()
	traits.SetEmail(email)

	service.enqueueMessage(segment.Identify{
		UserId: userID.String(),
		Traits: traits,
	})

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventAccountVerified,
		Properties: props,
	})

	service.hubspot.EnqueueEvent(email, service.satelliteName+"_"+eventAccountVerified, map[string]interface{}{
		"userid": userID.String(),
	})
}

// TrackEvent sends an arbitrary event associated with user ID to Segment.
// It is used for tracking occurrences of client-side events.
func (service *Service) TrackEvent(eventName string, userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	// do not track if the event name is an invalid client-side event
	if !service.clientEvents[eventName] {
		service.log.Error("Invalid client-triggered event", zap.String("eventName", eventName))
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventName,
		Properties: props,
	})

	service.hubspot.EnqueueEvent(email, service.satelliteName+"_"+eventName, map[string]interface{}{
		"userid": userID.String(),
	})
}

// TrackErrorEvent sends an arbitrary error event associated with user ID to Segment.
// It is used for tracking occurrences of client-side errors.
func (service *Service) TrackErrorEvent(userID uuid.UUID, email string, source string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)
	props.Set("source", source)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventError,
		Properties: props,
	})
}

// TrackLinkEvent sends an arbitrary event and link associated with user ID to Segment.
// It is used for tracking occurrences of client-side events.
func (service *Service) TrackLinkEvent(eventName string, userID uuid.UUID, email, link string) {
	if !service.config.Enabled {
		return
	}

	// do not track if the event name is an invalid client-side event
	if !service.clientEvents[eventName] {
		service.log.Error("Invalid client-triggered event", zap.String("eventName", eventName))
		return
	}

	props := segment.NewProperties()
	props.Set("link", link)
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventName,
		Properties: props,
	})

	service.hubspot.EnqueueEvent(email, service.satelliteName+"_"+eventName, map[string]interface{}{
		"userid": userID.String(),
		"link":   link,
	})
}

// TrackCreditCardAdded sends an "Credit Card Added" event to Segment.
func (service *Service) TrackCreditCardAdded(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventCreditCardAdded,
		Properties: props,
	})

}

// PageVisitEvent sends a page visit event associated with user ID to Segment.
// It is used for tracking occurrences of client-side events.
func (service *Service) PageVisitEvent(pageName string, userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)
	props.Set("path", pageName)
	props.Set("user_id", userID.String())
	props.Set("satellite", service.satelliteName)

	service.enqueueMessage(segment.Page{
		UserId:     userID.String(),
		Name:       "Page Requested",
		Properties: props,
	})

}

// TrackProjectLimitError sends an "Project Limit Error" event to Segment.
func (service *Service) TrackProjectLimitError(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventProjectLimitError,
		Properties: props,
	})

}

// TrackStorjTokenAdded sends an "Storj Token Added" event to Segment.
func (service *Service) TrackStorjTokenAdded(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventStorjTokenAdded,
		Properties: props,
	})

}

// TrackProjectMemberAddition sends an "Project Member Added" event to Segment.
func (service *Service) TrackProjectMemberAddition(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventProjectMemberAdded,
		Properties: props,
	})

}

// TrackProjectMemberDeletion sends an "Project Member Deleted" event to Segment.
func (service *Service) TrackProjectMemberDeletion(userID uuid.UUID, email string) {
	if !service.config.Enabled {
		return
	}

	props := segment.NewProperties()
	props.Set("email", email)

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      service.satelliteName + " " + eventProjectMemberDeleted,
		Properties: props,
	})

}
