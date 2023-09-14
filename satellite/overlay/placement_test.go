// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/common/storj/location"
	"storj.io/storj/satellite/nodeselection"
)

func TestPlacementFromString(t *testing.T) {
	signer, err := storj.NodeIDFromString("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4")
	require.NoError(t, err)

	t.Run("invalid country-code", func(t *testing.T) {
		p := NewPlacementRules()
		err := p.AddPlacementFromString(`1:country("ZZZZ")`)
		require.Error(t, err)
	})

	t.Run("country tests", func(t *testing.T) {
		countryTest := func(placementDef string, shouldBeIncluded []location.CountryCode, shouldBeExcluded []location.CountryCode) {
			p := NewPlacementRules()
			err := p.AddPlacementFromString("11:" + placementDef)
			require.NoError(t, err)
			filters := p.placements[storj.PlacementConstraint(11)]
			require.NotNil(t, filters)
			for _, code := range shouldBeExcluded {
				require.False(t, filters.Match(&nodeselection.SelectedNode{
					CountryCode: code,
				}), "%s shouldn't be included in placement %s", code, placementDef)
			}
			for _, code := range shouldBeIncluded {
				require.True(t, filters.Match(&nodeselection.SelectedNode{
					CountryCode: code,
				}), "%s is not included in placement %s", code, placementDef)
			}
		}
		countryTest(`country("GB")`, []location.CountryCode{location.UnitedKingdom}, []location.CountryCode{location.Germany, location.UnitedStates})
		countryTest(`country("EU")`, []location.CountryCode{location.Germany, location.Hungary}, []location.CountryCode{location.UnitedStates, location.Norway, location.Iceland})
		countryTest(`country("EEA")`, []location.CountryCode{location.Germany, location.Hungary, location.Norway, location.Iceland}, []location.CountryCode{location.UnitedStates})
		countryTest(`country("ALL","!EU")`, []location.CountryCode{location.Norway, location.India}, []location.CountryCode{location.Germany, location.Hungary})
		countryTest(`country("ALL", "!RU", "!BY")`, []location.CountryCode{location.Norway, location.India, location.UnitedStates}, []location.CountryCode{location.Russia, location.Belarus})

	})

	t.Run("tag rule", func(t *testing.T) {
		tagged := func(key string, value string) nodeselection.NodeTags {
			return nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   key,
					Value:  []byte(value),
				},
			}
		}

		testCases := []struct {
			name          string
			placement     string
			includedNodes []*nodeselection.SelectedNode
			excludedNodes []*nodeselection.SelectedNode
		}{
			{
				name:      "simple tag",
				placement: `11:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar")`,
				includedNodes: []*nodeselection.SelectedNode{
					{
						Tags: tagged("foo", "bar"),
					},
				},
				excludedNodes: []*nodeselection.SelectedNode{
					{
						CountryCode: location.Germany,
					},
				},
			},
			{
				name:      "tag not empty",
				placement: `11:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo",notEmpty())`,
				includedNodes: []*nodeselection.SelectedNode{
					{
						Tags: tagged("foo", "barx"),
					},
					{
						Tags: tagged("foo", "bar"),
					},
				},
				excludedNodes: []*nodeselection.SelectedNode{
					{
						Tags: tagged("foo", ""),
					},
					{
						CountryCode: location.Germany,
					},
				},
			},
			{
				name:      "tag empty",
				placement: `11:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo",empty())`,
				includedNodes: []*nodeselection.SelectedNode{
					{
						Tags: tagged("foo", ""),
					},
				},
				excludedNodes: []*nodeselection.SelectedNode{
					{
						Tags: tagged("foo", "bar"),
					},
					{
						CountryCode: location.Germany,
					},
				},
			},
		}
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				p := NewPlacementRules()
				err := p.AddPlacementFromString(tc.placement)
				require.NoError(t, err)
				filters := p.placements[storj.PlacementConstraint(11)]
				require.NotNil(t, filters)
				for _, i := range tc.includedNodes {
					require.True(t, filters.Match(i), "%v should be included", i)
				}
				for _, e := range tc.excludedNodes {
					require.False(t, filters.Match(e), "%v should be excluded", e)
				}
			})
		}
	})

	t.Run("placement reuse", func(t *testing.T) {
		p := NewPlacementRules()
		err := p.AddPlacementFromString(`1:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar");2:exclude(placement(1))`)
		require.NoError(t, err)

		require.True(t, p.placements[storj.PlacementConstraint(1)].Match(&nodeselection.SelectedNode{
			Tags: nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))

		placement2 := p.placements[storj.PlacementConstraint(2)]
		require.False(t, placement2.Match(&nodeselection.SelectedNode{
			Tags: nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))
		require.True(t, placement2.Match(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
		}))

	})

	t.Run("all rules", func(t *testing.T) {
		for _, syntax := range []string{
			`11:all(country("GB"),tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar"))`,
			`11:country("GB") && tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar")`,
		} {
			p := NewPlacementRules()
			err := p.AddPlacementFromString(syntax)
			require.NoError(t, err)
			filters := p.placements[storj.PlacementConstraint(11)]
			require.NotNil(t, filters)
			require.True(t, filters.Match(&nodeselection.SelectedNode{
				CountryCode: location.UnitedKingdom,
				Tags: nodeselection.NodeTags{
					{
						Signer: signer,
						Name:   "foo",
						Value:  []byte("bar"),
					},
				},
			}))
			require.False(t, filters.Match(&nodeselection.SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))
			require.False(t, filters.Match(&nodeselection.SelectedNode{
				CountryCode: location.Germany,
				Tags: nodeselection.NodeTags{
					{
						Signer: signer,
						Name:   "foo",
						Value:  []byte("bar"),
					},
				},
			}))
		}
		t.Run("invalid", func(t *testing.T) {
			p := NewPlacementRules()
			err := p.AddPlacementFromString("10:1 && 2")
			require.Error(t, err)
		})
	})

	t.Run("multi rule", func(t *testing.T) {
		p := NewPlacementRules()
		err := p.AddPlacementFromString(`11:country("GB");12:country("DE")`)
		require.NoError(t, err)

		filters := p.placements[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.True(t, filters.Match(&nodeselection.SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.False(t, filters.Match(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
		}))

		filters = p.placements[storj.PlacementConstraint(12)]
		require.NotNil(t, filters)
		require.False(t, filters.Match(&nodeselection.SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.True(t, filters.Match(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
		}))

	})

	t.Run("annotation usage", func(t *testing.T) {
		t.Run("normal", func(t *testing.T) {
			t.Parallel()
			p := NewPlacementRules()
			err := p.AddPlacementFromString(`11:annotated(country("GB"),annotation("autoExcludeSubnet","off"))`)
			require.NoError(t, err)
			filters := p.placements[storj.PlacementConstraint(11)]
			require.True(t, filters.Match(&nodeselection.SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))

			require.Equal(t, nodeselection.GetAnnotation(filters, "autoExcludeSubnet"), "off")
		})
		t.Run("with &&", func(t *testing.T) {
			t.Parallel()
			p := NewPlacementRules()
			err := p.AddPlacementFromString(`11:country("GB") && annotation("foo","bar") && annotation("bar","foo")`)
			require.NoError(t, err)

			filters := p.placements[storj.PlacementConstraint(11)]
			require.True(t, filters.Match(&nodeselection.SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))
			require.Equal(t, "bar", nodeselection.GetAnnotation(filters, "foo"))
			require.Equal(t, "foo", nodeselection.GetAnnotation(filters, "bar"))
			require.Equal(t, "", nodeselection.GetAnnotation(filters, "kossuth"))
		})
		t.Run("chained", func(t *testing.T) {
			t.Parallel()
			p := NewPlacementRules()
			err := p.AddPlacementFromString(`11:annotated(annotated(country("GB"),annotation("foo","bar")),annotation("bar","foo"))`)
			require.NoError(t, err)
			filters := p.placements[storj.PlacementConstraint(11)]
			require.True(t, filters.Match(&nodeselection.SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))

			require.Equal(t, "bar", nodeselection.GetAnnotation(filters, "foo"))
			require.Equal(t, "foo", nodeselection.GetAnnotation(filters, "bar"))
			require.Equal(t, "", nodeselection.GetAnnotation(filters, "kossuth"))
		})
		t.Run("location", func(t *testing.T) {
			p := NewPlacementRules()
			s := fmt.Sprintf(`11:annotated(annotated(country("GB"),annotation("%s","test-location")),annotation("%s","%s"))`, nodeselection.Location, nodeselection.AutoExcludeSubnet, nodeselection.AutoExcludeSubnetOFF)
			require.NoError(t, p.AddPlacementFromString(s))
			filters := p.placements[storj.PlacementConstraint(11)]
			require.True(t, filters.Match(&nodeselection.SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))

			require.Equal(t, nodeselection.AutoExcludeSubnetOFF, nodeselection.GetAnnotation(filters, nodeselection.AutoExcludeSubnet))
			require.Equal(t, "test-location", nodeselection.GetAnnotation(filters, nodeselection.Location))
		})
	})

	t.Run("exclude", func(t *testing.T) {
		p := NewPlacementRules()
		err := p.AddPlacementFromString(`11:exclude(country("GB"))`)
		require.NoError(t, err)
		filters := p.placements[storj.PlacementConstraint(11)]
		require.False(t, filters.Match(&nodeselection.SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.True(t, filters.Match(&nodeselection.SelectedNode{
			CountryCode: location.Germany,
		}))
	})

	t.Run("legacy geofencing rules", func(t *testing.T) {
		p := NewPlacementRules()
		p.AddLegacyStaticRules()

		t.Run("nr", func(t *testing.T) {
			nr := p.placements[storj.NR]
			require.True(t, nr.Match(&nodeselection.SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))
			require.False(t, nr.Match(&nodeselection.SelectedNode{
				CountryCode: location.Russia,
			}))
			require.False(t, nr.Match(&nodeselection.SelectedNode{
				CountryCode: 0,
			}))
		})
		t.Run("us", func(t *testing.T) {
			us := p.placements[storj.US]
			require.True(t, us.Match(&nodeselection.SelectedNode{
				CountryCode: location.UnitedStates,
			}))
			require.False(t, us.Match(&nodeselection.SelectedNode{
				CountryCode: location.Germany,
			}))
			require.False(t, us.Match(&nodeselection.SelectedNode{
				CountryCode: 0,
			}))
		})

	})

	t.Run("full example", func(t *testing.T) {
		// this is a realistic configuration, compatible with legacy rules + using one node tag for specific placement

		rules1 := NewPlacementRules()
		err := rules1.AddPlacementFromString(`
						10:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","selected",notEmpty());
						11:placement(10) && annotation("autoExcludeSubnet","off") && annotation("location","do-not-use");
						12:placement(10) && annotation("autoExcludeSubnet","off") && country("US") && annotation("location","us-select-1");
						0:exclude(placement(10)) && annotation("location","global");
						1:country("EU") && exclude(placement(10)) && annotation("location","eu-1");
						2:country("EEA") && exclude(placement(10)) && annotation("location","eea-1");
						3:country("US") && exclude(placement(10)) && annotation("location","us-1");
						4:country("DE") && exclude(placement(10)) && annotation("location","de-1");
						6:country("*","!BY", "!RU", "!NONE") && exclude(placement(10)) && annotation("location","custom-1");`)
		require.NoError(t, err)

		// for countries, it should be the same as above
		rules2 := NewPlacementRules()
		rules2.AddLegacyStaticRules()

		testCountries := []location.CountryCode{
			location.Russia,
			location.India,
			location.Belarus,
			location.UnitedStates,
			location.Canada,
			location.Brazil,
			location.Ghana,
		}
		testCountries = append(testCountries, nodeselection.EeaCountriesWithoutEu...)
		testCountries = append(testCountries, nodeselection.EuCountries...)

		// check if old geofencing rules are working as before (and string based config is the same as the code base)
		for _, placement := range []storj.PlacementConstraint{storj.EU, storj.EEA, storj.DE, storj.US, storj.NR} {
			filter1 := rules1.CreateFilters(placement)
			filter2 := rules2.CreateFilters(placement)
			for _, country := range testCountries {
				result1 := filter1.Match(&nodeselection.SelectedNode{
					CountryCode: country,
				})
				result2 := filter2.Match(&nodeselection.SelectedNode{
					CountryCode: country,
				})
				assert.Equal(t, result1, result2, "default legacy rules do not match string based configuration for placement %d and country %s", placement, country)
			}
		}

		// make sure that new rules exclude location.None from NR
		assert.False(t, rules1.CreateFilters(storj.NR).Match(&nodeselection.SelectedNode{}))
		assert.False(t, rules2.CreateFilters(storj.NR).Match(&nodeselection.SelectedNode{}))

		// make sure tagged nodes (even from EU) matches only the special placement
		node := &nodeselection.SelectedNode{
			CountryCode: location.Germany,
			Tags: nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   "selected",
					Value:  []byte("true"),
				},
			},
		}

		for _, placement := range []storj.PlacementConstraint{storj.EveryCountry, storj.EU, storj.EEA, storj.DE, storj.US, storj.NR} {
			assert.False(t, rules1.CreateFilters(placement).Match(node))
		}
		assert.False(t, rules1.CreateFilters(6).Match(node))

		// any value is accepted
		assert.True(t, rules1.CreateFilters(11).Match(&nodeselection.SelectedNode{
			Tags: nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   "selected",
					Value:  []byte("true,something"),
				},
			},
		}))

		// but not empty
		assert.False(t, rules1.CreateFilters(11).Match(&nodeselection.SelectedNode{
			Tags: nodeselection.NodeTags{
				{
					Signer: signer,
					Name:   "selected",
					Value:  []byte(""),
				},
			},
		}))

		// check if annotation present on 11,12, but not on other
		for i := 0; i < 20; i++ {
			subnetDisabled := nodeselection.GetAnnotation(rules1.CreateFilters(storj.PlacementConstraint(i)), nodeselection.AutoExcludeSubnet) == nodeselection.AutoExcludeSubnetOFF
			if i == 11 || i == 12 {
				require.True(t, subnetDisabled, "Placement filter should be disabled for %d", i)
			} else {
				require.False(t, subnetDisabled, "Placement filter should be enabled for %d", i)
			}
		}

	})

}
