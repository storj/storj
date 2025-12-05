// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hubspotmails

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
)

// Config holds the configuration for the HubSpot email service.
type Config struct {
	Enabled        bool           `help:"indicates whether the hubspot email service is enabled" default:"false"`
	DefaultTimeout time.Duration  `help:"the default timeout for the hubspot http client" default:"10s"`
	SendEmailAPI   string         `help:"hubspot send email API endpoint" default:"https://api.hubapi.com/marketing/v3/transactional/single-email/send"`
	EmailKindIDMap EmailKindIDMap `help:"a map of email kinds to their corresponding IDs in a format kind:id;kind1:id1" default:""`
}

// Ensure that EmailKindIDMap implements pflag.Value.
var _ pflag.Value = (*EmailKindIDMap)(nil)

// EmailKindIDMap is a map of email kinds to their corresponding IDs.
type EmailKindIDMap struct {
	kindIDMap map[MailKind]int64
}

// Type returns the type of the pflag.Value.
func (*EmailKindIDMap) Type() string { return "analytics.EmailKindIDMap" }

// String returns the string representation of the EmailKindIDMap.
func (m *EmailKindIDMap) String() string {
	if m == nil {
		return ""
	}

	var s strings.Builder
	left := len(m.kindIDMap)
	for email, id := range m.kindIDMap {
		s.WriteString(fmt.Sprintf("%s:%d", email, id))
		left--
		if left > 0 {
			s.WriteRune(';')
		}
	}
	return s.String()
}

// Set sets the list of kind-IDs from a string representation.
func (m *EmailKindIDMap) Set(s string) error {
	parsed := make(map[MailKind]int64)
	for _, kindIDStr := range strings.Split(s, ";") {
		if kindIDStr == "" {
			continue
		}

		info := strings.Split(kindIDStr, ":")
		if len(info) != 2 {
			return Error.New("Invalid kind-ID pair (expected format kind:id got %s)", kindIDStr)
		}

		kind := strings.TrimSpace(info[0])
		if len(kind) == 0 {
			return Error.New("Kind must not be empty")
		}

		id, err := strconv.ParseInt(info[1], 10, 64)
		if err != nil {
			return Error.Wrap(err)
		}

		parsed[MailKind(kind)] = id
	}
	m.kindIDMap = parsed
	return nil
}

// Get an ID for the given email kind.
func (m *EmailKindIDMap) Get(kind MailKind) (int64, error) {
	if id, ok := m.kindIDMap[kind]; ok {
		return id, nil
	}

	return 0, errs.New("no matching ID for (%s)", kind)
}
