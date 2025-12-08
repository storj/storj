// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventingconfig

import (
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
)

// ProjectSet is a set of project UUIDs that are enabled for bucket eventing.
type ProjectSet map[uuid.UUID]struct{}

// Type returns the type of the ProjectSet.
func (s ProjectSet) Type() string {
	return "eventing.ProjectSet"
}

// Set sets the value of the ProjectSet from a comma-separated string of project UUIDs.
func (s *ProjectSet) Set(value string) error {
	if value == "" {
		*s = map[uuid.UUID]struct{}{}
		return nil
	}

	parts := strings.Split(value, ",")
	*s = make(map[uuid.UUID]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		projectID, err := uuid.FromString(part)
		if err != nil {
			return errs.New("invalid project ID: %q: %w", part, err)
		}

		(*s)[projectID] = struct{}{}
	}
	return nil
}

// String returns the string representation of the ProjectSet.
func (s ProjectSet) String() string {
	if len(s) == 0 {
		return ""
	}

	var b strings.Builder
	i := 0
	for projectID := range s {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(projectID.String())
		i++
	}
	return b.String()
}

// Enabled checks if the given project ID is enabled for bucket eventing.
func (s ProjectSet) Enabled(projectID uuid.UUID) bool {
	_, ok := s[projectID]
	return ok
}
