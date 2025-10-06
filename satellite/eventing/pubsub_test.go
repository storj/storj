// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseTopicName(t *testing.T) {
	var tests = []struct {
		name              string
		input             string
		expectedProjectID string
		expectedTopicID   string
		expectError       bool
	}{
		{
			name:              "valid_topic_name",
			input:             "projects/my-gcp-project/topics/my-test-topic",
			expectedProjectID: "my-gcp-project",
			expectedTopicID:   "my-test-topic",
			expectError:       false,
		},
		{
			name:              "valid_topic_name_with_dashes",
			input:             "projects/another-project-123/topics/topic-with-dashes",
			expectedProjectID: "another-project-123",
			expectedTopicID:   "topic-with-dashes",
			expectError:       false,
		},
		{
			name:              "invalid_format_leading_slash",
			input:             "/projects/my-gcp-project/topics/my-test-topic",
			expectedProjectID: "",
			expectedTopicID:   "",
			expectError:       true,
		},
		{
			name:              "invalid_format_too_few_parts",
			input:             "projects/my-gcp-project/topics",
			expectedProjectID: "",
			expectedTopicID:   "",
			expectError:       true,
		},
		{
			name:              "invalid_format_wrong_prefix",
			input:             "organizations/my-org/topics/my-topic",
			expectedProjectID: "",
			expectedTopicID:   "",
			expectError:       true,
		},
		{
			name:              "invalid_format_wrong_middle",
			input:             "projects/my-gcp-project/subscriptions/my-sub",
			expectedProjectID: "",
			expectedTopicID:   "",
			expectError:       true,
		},
		{
			name:              "empty_string",
			input:             "",
			expectedProjectID: "",
			expectedTopicID:   "",
			expectError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectID, topicID, err := ParseTopicName(tt.input)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedProjectID, projectID)
			require.Equal(t, tt.expectedTopicID, topicID)
		})
	}
}
