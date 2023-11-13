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

	t.Run("All nested structs and slices", func(t *testing.T) {
		type Item struct{}
		types := NewTypes()
		types.Register(
			typeCustomName{
				Type: reflect.TypeOf(struct {
					Name      string
					Addresses []struct {
						Address string
						PO      string
					}
					Job struct {
						Company      string
						Position     string
						StartingYear uint
					}
					Documents []struct {
						Path       string
						Content    string
						Valoration testTypesValoration
					}
					Items []Item
				}{}),
				name: "Response",
			})

		allTypes := types.All()
		require.Len(t, allTypes, 10, "total number of types")

		typesNames := []string{}
		for _, name := range allTypes {
			typesNames = append(typesNames, name)
		}

		require.ElementsMatch(t, []string{
			"string", "uint",
			"Response",
			"ResponseAddresses", "ResponseAddressesItem",
			"ResponseJob",
			"ResponseDocuments", "ResponseDocumentsItem", "testTypesValoration",
			"Item",
		}, typesNames)
	})

	t.Run("All panic types without unique names", func(t *testing.T) {
		types := NewTypes()
		types.Register(typeCustomName{
			Type: reflect.TypeOf(struct {
				Name      string
				Addresses []struct {
					Address string
					PO      string
				}
				Job struct {
					Company      string
					Position     string
					StartingYear uint
				}
				Documents []struct {
					Path       string
					Content    string
					Valoration testTypesValoration
				}
			}{}),
			name: "Response",
		})

		types.Register(typeCustomName{
			Type: reflect.TypeOf(struct {
				Reference string
			}{}),
			name: "Response",
		})

		require.Panics(t, func() {
			types.All()
		})
	})
}
