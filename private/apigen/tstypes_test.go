// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

type testTypesValoration struct {
	Points uint
}

func TestTypes(t *testing.T) {
	t.Run("Register panics with some anonymous types", func(t *testing.T) {
		types := NewTypes()
		require.Panics(t, func() {
			types.Register(reflect.TypeOf([2]struct{}{}))
		}, "array")

		require.Panics(t, func() {
			types.Register(reflect.TypeOf([]struct{}{}))
		}, "slice")

		require.Panics(t, func() {
			types.Register(reflect.TypeOf(struct{}{}))
		}, "struct")
	})

	t.Run("All returns nested types", func(t *testing.T) {
		typesList := []reflect.Type{
			reflect.TypeOf(true),
			reflect.TypeOf(int64(10)),
			reflect.TypeOf(uint8(9)),
			reflect.TypeOf(float64(99.9)),
			reflect.TypeOf("this is a test"),
			reflect.TypeOf(testTypesValoration{}),
		}

		types := NewTypes()
		for _, li := range typesList {
			types.Register(li)
		}

		allTypes := types.All()

		require.Len(t, allTypes, 7, "total number of types")
		require.Subset(t, allTypes, typesList, "all types contains at least the registered ones")
	})

	t.Run("Anonymous types panics", func(t *testing.T) {
		type Address struct {
			Address string
			PO      string
		}
		type Job struct {
			Company         string
			Position        string
			StartingYear    uint
			ContractClauses []struct { // This is what it makes Types.All to panic
				ClauseID  uint
				CauseDesc string
			}
		}

		type Citizen struct {
			Name      string
			Addresses []Address
			Job       Job
		}

		types := NewTypes()
		types.Register(reflect.TypeOf(Citizen{}))
		require.Panics(t, func() {
			types.All()
		})
	})
}
