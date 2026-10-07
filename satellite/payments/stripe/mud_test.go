// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/storj/satellite/console"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/mud"
)

func TestModuleClient(t *testing.T) {
	for _, tt := range []struct {
		name      string
		selection string
		expected  Client
	}{
		{name: "default", selection: "", expected: &stripeClient{}},
		{name: "mock", selection: "stripe.Client=*stripe.mockStripeClient", expected: &mockStripeClient{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := testcontext.New(t)

			ball := mud.NewBall()
			mud.Supply[*zap.Logger](ball, zaptest.NewLogger(t))
			mud.Supply[Config](ball, Config{})
			mud.Supply[DB](ball, customersOnlyDB{})
			mud.Supply[console.Users](ball, usersStub{})
			clientModule(ball)

			// the same selection as the --components flag of the modular satellite
			modular.CreateSelectorFromString(ball, tt.selection)

			require.NoError(t, mud.ForEachDependency(ball, mud.Select[Client](ball), mud.Initialize(ctx)))
			require.IsType(t, tt.expected, mud.MustLookup[Client](ball))
		})
	}
}

// customersOnlyDB is a DB which only supports Customers(), the only method used by the mock client.
type customersOnlyDB struct{ DB }

func (customersOnlyDB) Customers() CustomersDB { return nil }

type usersStub struct{ console.Users }
