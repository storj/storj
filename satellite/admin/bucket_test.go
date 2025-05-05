// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/uuid"
)

func TestValidateRequestParameters(t *testing.T) {
	uid, err := uuid.New()
	require.NoError(t, err, "failed to generate uuid")

	testCases := []struct {
		name   string
		params map[string]string
		// expectations
		project uuid.NullUUID
		bucket  []byte
		err     string
	}{
		{"missing project", map[string]string{}, uuid.NullUUID{}, nil, "project-uuid missing"},
		{
			name: "invalid project",
			params: map[string]string{
				"project": "invalidUUID",
			},
			project: uuid.NullUUID{},
			bucket:  nil,
			err:     "project-uuid is not a valid uuid",
		},
		{
			name: "missing bucket",
			params: map[string]string{
				"project": uid.String(),
			},
			project: uuid.NullUUID{
				UUID:  uid,
				Valid: true,
			},
			bucket: nil,
			err:    "bucket name is missing",
		},
		{
			name: "empty bucket",
			params: map[string]string{
				"project": uid.String(),
				"bucket":  "",
			},
			project: uuid.NullUUID{
				UUID:  uid,
				Valid: true,
			},
			bucket: nil,
			err:    "bucket name is missing",
		},
		{
			name: "valid parameters",
			params: map[string]string{
				"project": uid.String(),
				"bucket":  "test-bucket",
			},
			project: uuid.NullUUID{
				UUID:  uid,
				Valid: true,
			},
			bucket: []byte("test-bucket"),
			err:    "",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			project, bucket, err := validateBucketPathParameters(testCase.params)

			require.Equal(t, testCase.project, project)
			require.Equal(t, testCase.bucket, bucket)

			if len(testCase.err) > 0 {
				require.Error(t, err)
				require.Equal(t, testCase.err, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
