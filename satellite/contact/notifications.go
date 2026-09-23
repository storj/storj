// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"net/url"

	"storj.io/common/pb"
)

const (
	maxNotificationTitleLength   = 256
	maxNotificationMessageLength = 4096
	maxNotificationLinkLength    = 2048
)

// Notification is a custom message, defined in the satellite configuration, which is
// delivered to the storage nodes during check-in and shown on their web interface.
type Notification struct {
	// Key is a stable identifier of the notification. Nodes derive the primary key of the
	// stored notification from it, therefore editing Title or Message updates the
	// notification the nodes already received, while changing Key delivers a new one.
	Key     string `yaml:"key"`
	Title   string `yaml:"title"`
	Message string `yaml:"message"`

	// Link is an optional supporting URL, shown next to the message. It must use https,
	// as the storage nodes render it as a link on their web interface.
	Link string `yaml:"link"`
	// LinkLabel is the text of the link. The URL itself is shown when it's empty.
	LinkLabel string `yaml:"link-label"`
}

// validateNotifications checks the configured notifications and converts them to their
// protobuf form. Notifications are validated during startup instead of during check-in:
// a malformed configuration should stop the satellite, not reach every storage node.
func validateNotifications(notifications []Notification) ([]*pb.Notification, error) {
	if len(notifications) == 0 {
		return nil, nil
	}

	converted := make([]*pb.Notification, 0, len(notifications))
	keys := make(map[string]struct{}, len(notifications))

	for i, notification := range notifications {
		switch {
		case notification.Key == "":
			return nil, Error.New("notification #%d: key is required", i)
		case notification.Title == "":
			return nil, Error.New("notification %q: title is required", notification.Key)
		case len(notification.Title) > maxNotificationTitleLength:
			return nil, Error.New("notification %q: title is too long (%d characters, maximum is %d)",
				notification.Key, len(notification.Title), maxNotificationTitleLength)
		case len(notification.Message) > maxNotificationMessageLength:
			return nil, Error.New("notification %q: message is too long (%d characters, maximum is %d)",
				notification.Key, len(notification.Message), maxNotificationMessageLength)
		}

		if _, exists := keys[notification.Key]; exists {
			return nil, Error.New("notification %q: duplicate key", notification.Key)
		}
		keys[notification.Key] = struct{}{}

		if err := validateLink(notification); err != nil {
			return nil, err
		}

		converted = append(converted, &pb.Notification{
			Key:       notification.Key,
			Title:     notification.Title,
			Message:   notification.Message,
			Link:      notification.Link,
			LinkLabel: notification.LinkLabel,
		})
	}

	return converted, nil
}

// validateLink checks the optional supporting link of a notification. Only https is
// accepted: the storage nodes render the link on their web interface, so schemes like
// javascript: would turn a notification into a way to run code in the browser of the
// node operator.
func validateLink(notification Notification) error {
	if notification.Link == "" {
		if notification.LinkLabel != "" {
			return Error.New("notification %q: link label is set without a link", notification.Key)
		}
		return nil
	}

	if len(notification.Link) > maxNotificationLinkLength {
		return Error.New("notification %q: link is too long (%d characters, maximum is %d)",
			notification.Key, len(notification.Link), maxNotificationLinkLength)
	}
	if len(notification.LinkLabel) > maxNotificationTitleLength {
		return Error.New("notification %q: link label is too long (%d characters, maximum is %d)",
			notification.Key, len(notification.LinkLabel), maxNotificationTitleLength)
	}

	parsed, err := url.Parse(notification.Link)
	if err != nil {
		return Error.New("notification %q: link is not a valid URL: %v", notification.Key, err)
	}
	if parsed.Scheme != "https" {
		return Error.New("notification %q: link must use https, got %q", notification.Key, parsed.Scheme)
	}
	if parsed.Host == "" {
		return Error.New("notification %q: link has no host", notification.Key)
	}

	return nil
}
