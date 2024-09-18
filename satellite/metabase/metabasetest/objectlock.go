// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package metabasetest

import (
	"testing"
	"time"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
)

// ObjectLockDeletionTestRunner runs object deletion tests with different Object Lock test cases.
type ObjectLockDeletionTestRunner struct {
	// TestProtected is run with Object Lock options that are expected to prohibit object deletion.
	TestProtected func(t *testing.T, testCase ObjectLockDeletionTestCase)
	// TestRemovable is run with Object Lock options that are not expected to prohibit object deletion.
	TestRemovable func(t *testing.T, testCase ObjectLockDeletionTestCase)
}

// ObjectLockDeletionTestCase contains configuration options for an Object Lock deletion test case.
type ObjectLockDeletionTestCase struct {
	// Retention is the retention configuration of the object created for this test case.
	Retention metabase.Retention
	// LegalHold is the legal hold status of the object created for this test case.
	LegalHold bool
	// BypassGovernance indicates whether governance mode bypass should be enabled when deleting
	// the object created for this test case.
	BypassGovernance bool
}

// Run runs the Object Lock deletion test cases.
func (opts ObjectLockDeletionTestRunner) Run(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Minute)

	type namedTestCase struct {
		name     string
		testCase ObjectLockDeletionTestCase
	}

	for _, tt := range []namedTestCase{
		{
			name: "Compliance (active)",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: future,
				},
			},
		}, {
			name: "Compliance (active) - Governance bypass",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: future,
				},
				BypassGovernance: true,
			},
		}, {
			name: "Governance (active)",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: future,
				},
			},
		}, {
			name: "Legal hold",
			testCase: ObjectLockDeletionTestCase{
				LegalHold: true,
			},
		}, {
			name: "Legal hold and compliance (active)",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: future,
				},
				LegalHold: true,
			},
		}, {
			name: "Legal hold and compliance (expired)",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: past,
				},
				LegalHold: true,
			},
		}, {
			name: "Legal hold and governance (active)",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: future,
				},
				LegalHold: true,
			},
		}, {
			name: "Legal hold and governance (active) - Governance bypass",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: future,
				},
				BypassGovernance: true,
				LegalHold:        true,
			},
		}, {
			name: "Legal hold and governance (expired)",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: past,
				},
				LegalHold: true,
			},
		},
	} {
		t.Run("Protected Object Lock configuration - "+tt.name, func(t *testing.T) {
			opts.TestProtected(t, tt.testCase)
		})
	}

	for _, tt := range []namedTestCase{
		{
			name: "Compliance (expired)",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.ComplianceMode,
					RetainUntil: past,
				},
			},
		}, {
			name: "Governance (expired)",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: past,
				},
			},
		}, {
			name: "Governance (active) - Governance bypass",
			testCase: ObjectLockDeletionTestCase{
				Retention: metabase.Retention{
					Mode:        storj.GovernanceMode,
					RetainUntil: future,
				},
				BypassGovernance: true,
			},
		},
	} {
		t.Run("Removable Object Lock configuration - "+tt.name, func(t *testing.T) {
			opts.TestRemovable(t, tt.testCase)
		})
	}
}
