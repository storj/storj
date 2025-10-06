// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.
package eventingconfig

import (
	"fmt"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// Config specifies the buckets with enabled eventing and their destinations.
type Config struct {
	Buckets BucketLocationTopicIDMap `help:"defines which buckets are monitored for events (comma separated list of \"project_id:bucket_name:topic_id\")" default:""`
}

// BucketLocationTopicIDMap is a map of bucket locations to their corresponding topic ID values.
type BucketLocationTopicIDMap map[metabase.BucketLocation]string

// Type returns the type of the BucketLocationTopicIDMap.
func (m BucketLocationTopicIDMap) Type() string {
	return "eventing.BucketLocationTopicIDMap"
}

// Set sets the value of the BucketLocationTopicIDMap from a string.
func (m *BucketLocationTopicIDMap) Set(s string) error {
	if s == "" {
		*m = map[metabase.BucketLocation]string{}
		return nil
	}

	parts := strings.Split(s, ",")
	*m = make(map[metabase.BucketLocation]string, len(parts))
	for _, part := range parts {
		kv := strings.Split(part, ":")
		if len(kv) != 3 {
			return errs.New("invalid bucket tuple: %v", part)
		}

		projectID, err := uuid.FromString(kv[0])
		if err != nil {
			return errs.New("invalid project ID: %q: %w", kv[0], err)
		}

		loc := metabase.BucketLocation{
			ProjectID:  projectID,
			BucketName: metabase.BucketName(kv[1]),
		}

		if err := loc.Verify(); err != nil {
			return errs.New("invalid bucket location: %q: %w", kv[0], err)
		}

		(*m)[loc] = kv[2]
	}
	return nil
}

// String returns the string representation of the BucketLocationTopicIDMap.
func (m BucketLocationTopicIDMap) String() string {
	var b strings.Builder
	i := 0
	for loc, id := range m {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(fmt.Sprintf("%s:%s:%s", loc.ProjectID, loc.BucketName, id))
		i++
	}
	return b.String()
}

// Enabled checks if feature is enabled for the given project and bucket.
func (m BucketLocationTopicIDMap) Enabled(projectID uuid.UUID, bucketName string) bool {
	_, ok := m[metabase.BucketLocation{
		ProjectID:  projectID,
		BucketName: metabase.BucketName(bucketName),
	}]
	return ok
}

// GetTopicID returns the topic ID for the given project and bucket.
func (m BucketLocationTopicIDMap) GetTopicID(projectID uuid.UUID, bucketName string) string {
	return m[metabase.BucketLocation{
		ProjectID:  projectID,
		BucketName: metabase.BucketName(bucketName),
	}]
}
