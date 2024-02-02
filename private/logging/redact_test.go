// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedacted(t *testing.T) {
	require.Equal(t, "cockroach://root@localhost:26257/env1?sslmode=disable", Redacted("cockroach://root@localhost:26257/env1?sslmode=disable"))
	require.Equal(t, "cockroach://root:xxxxx@localhost:26257/env1?sslmode=disable", Redacted("cockroach://root:mypassword@localhost:26257/env1?sslmode=disable"))
}
