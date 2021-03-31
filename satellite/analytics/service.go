// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"go.uber.org/zap"
	segment "gopkg.in/segmentio/analytics-go.v3"

	"storj.io/common/uuid"
)

const (
	eventAccountCreated     = "Account Created"
	eventSignedIn           = "Signed In"
	eventProjectCreated     = "Project Created"
	eventAccessGrantCreated = "Access Grant Created"
)

// Config is a configuration struct for analytics Service.
type Config struct {
	SegmentWriteKey string `help:"segment write key" default:""`
	Enabled         bool   `help:"enable analytics reporting" default:"false"`
}

// Service for sending analytics.
//
// architecture: Service
type Service struct {
	log           *zap.Logger
	config        Config
	satelliteName string

	segment segment.Client
}

// NewService creates new service for creating sending analytics.
func NewService(log *zap.Logger, config Config, satelliteName string) *Service {
	service := &Service{
		log:           log,
		config:        config,
		satelliteName: satelliteName,
	}
	if config.Enabled {
		service.segment = segment.New(config.SegmentWriteKey)
	}
	return service
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
	ID            uuid.UUID
	FullName      string
	Email         string
	Type          UserType
	EmployeeCount string
	CompanyName   string
	JobTitle      string
}

func (service *Service) enqueueMessage(message segment.Message) {
	if !service.config.Enabled {
		return
	}

	err := service.segment.Enqueue(message)
	if err != nil {
		service.log.Error("Error enqueueing message", zap.Error(err))
	}
}

// TrackCreateUser sends an "Account Created" event to Segment.
func (service *Service) TrackCreateUser(fields TrackCreateUserFields) {
	traits := segment.NewTraits()
	traits.SetName(fields.FullName)
	traits.SetEmail(fields.Email)

	service.enqueueMessage(segment.Identify{
		UserId: fields.ID.String(),
		Traits: traits,
	})

	props := segment.NewProperties()
	props.Set("email", fields.Email)
	props.Set("name", fields.FullName)
	props.Set("satellite_selected", service.satelliteName)
	props.Set("account_type", fields.Type)

	if fields.Type == Professional {
		props.Set("company_size", fields.EmployeeCount)
		props.Set("company_name", fields.CompanyName)
		props.Set("job_title", fields.JobTitle)
	}

	service.enqueueMessage(segment.Track{
		UserId:     fields.ID.String(),
		Event:      eventAccountCreated,
		Properties: props,
	})
}

// TrackSignedIn sends an "Signed In" event to Segment.
func (service *Service) TrackSignedIn(userID uuid.UUID, email string) {
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
		Event:      eventSignedIn,
		Properties: props,
	})
}

// TrackProjectCreated sends an "Project Created" event to Segment.
func (service *Service) TrackProjectCreated(userID, projectID uuid.UUID, currentProjectCount int) {

	props := segment.NewProperties()
	props.Set("project_count", currentProjectCount)
	props.Set("project_id", projectID.String())

	service.enqueueMessage(segment.Track{
		UserId:     userID.String(),
		Event:      eventProjectCreated,
		Properties: props,
	})
}

// TrackAccessGrantCreated sends an "Access Grant Created" event to Segment.
func (service *Service) TrackAccessGrantCreated(userID uuid.UUID) {

	service.enqueueMessage(segment.Track{
		UserId: userID.String(),
		Event:  eventAccessGrantCreated,
	})
}
