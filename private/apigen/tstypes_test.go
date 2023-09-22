// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package apigen

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypes(t *testing.T) {
	t.Run("All returns nested types", func(t *testing.T) {
		typesList := []reflect.Type{
			reflect.TypeOf(true),
			reflect.TypeOf(int64(10)),
			reflect.TypeOf(uint8(9)),
			reflect.TypeOf(float64(99.9)),
			reflect.TypeOf("this is a test"),
			reflect.TypeOf(struct {
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
			}{}),
			reflect.TypeOf([]string{}),
			reflect.TypeOf([]struct {
				Path    string
				content string
			}{}),
		}

		types := NewTypes()
		for _, li := range typesList {
			types.Register(li)
		}

		allTypes := types.All()

		require.Len(t, allTypes, 13, "total number of types")
		require.Subset(t, allTypes, typesList, "all types contains at least the registered ones")
	})

	t.Run("anonymous structs", func(t *testing.T) {
		typesList := []reflect.Type{
			reflect.TypeOf(struct {
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
					Path    string
					Content string
				}
			}{}),
		}

		types := NewTypes()
		for _, li := range typesList {
			types.Register(li)
		}

		allTypes := types.All()

		require.Len(t, allTypes, 8, "total number of types")
		require.Subset(t, allTypes, typesList, "all types contains at least the registered ones")

		typesNames := []string{}
		for _, tp := range allTypes {
			typesNames = append(typesNames, tp.Name())
		}

		require.ElementsMatch(t, []string{"", "", "", "", "", "", "string", "uint"}, typesNames)
	})
}
