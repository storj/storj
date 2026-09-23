// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateNotifications(t *testing.T) {
	valid := Notification{Key: "maintenance", Title: "Maintenance", Message: "body"}

	for _, tt := range []struct {
		name          string
		notifications []Notification
		errContains   string
	}{
		{
			name:          "empty is allowed",
			notifications: nil,
		},
		{
			name:          "valid",
			notifications: []Notification{valid, {Key: "other", Title: "Other", Message: "body"}},
		},
		{
			name:          "message may be empty",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance"}},
		},
		{
			name:          "missing key",
			notifications: []Notification{{Title: "Maintenance", Message: "body"}},
			errContains:   "key is required",
		},
		{
			name:          "duplicate key",
			notifications: []Notification{valid, valid},
			errContains:   "duplicate",
		},
		{
			name:          "missing title",
			notifications: []Notification{{Key: "maintenance", Message: "body"}},
			errContains:   "title is required",
		},
		{
			name:          "title too long",
			notifications: []Notification{{Key: "maintenance", Title: strings.Repeat("a", maxNotificationTitleLength+1), Message: "body"}},
			errContains:   "title is too long",
		},
		{
			name:          "message too long",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Message: strings.Repeat("a", maxNotificationMessageLength+1)}},
			errContains:   "message is too long",
		},
		{
			name:          "with link",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Link: "https://forum.storj.io/t/1", LinkLabel: "Forum thread"}},
		},
		{
			name:          "link without label",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Link: "https://forum.storj.io/t/1"}},
		},
		{
			name:          "label without link",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", LinkLabel: "Forum thread"}},
			errContains:   "link label is set without a link",
		},
		{
			name:          "plain http link",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Link: "http://forum.storj.io/t/1"}},
			errContains:   "link must use https",
		},
		{
			name:          "javascript link",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Link: "javascript:alert(1)"}},
			errContains:   "link must use https",
		},
		{
			name:          "link without host",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Link: "https:///path"}},
			errContains:   "link has no host",
		},
		{
			name:          "unparsable link",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Link: "https://exa mple.com/\x7f"}},
			errContains:   "link is not a valid URL",
		},
		{
			name:          "link too long",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Link: "https://storj.io/" + strings.Repeat("a", maxNotificationLinkLength)}},
			errContains:   "link is too long",
		},
		{
			name:          "link label too long",
			notifications: []Notification{{Key: "maintenance", Title: "Maintenance", Link: "https://storj.io/", LinkLabel: strings.Repeat("a", maxNotificationTitleLength+1)}},
			errContains:   "link label is too long",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			converted, err := validateNotifications(tt.notifications)
			if tt.errContains != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			require.Len(t, converted, len(tt.notifications))
			for i, n := range tt.notifications {
				require.Equal(t, n.Key, converted[i].Key)
				require.Equal(t, n.Title, converted[i].Title)
				require.Equal(t, n.Message, converted[i].Message)
				require.Equal(t, n.Link, converted[i].Link)
				require.Equal(t, n.LinkLabel, converted[i].LinkLabel)
			}
		})
	}
}
