// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/storj"
	"storj.io/storj/shared/location"
)

func TestPlacementFromString(t *testing.T) {
	signer, err := storj.NodeIDFromString("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4")
	require.NoError(t, err)

	t.Run("invalid country-code", func(t *testing.T) {
		p := TestPlacementDefinitions()
		err := p.AddPlacementFromString(`1:country("ZZZZ")`)
		require.Error(t, err)
	})

	t.Run("country tests", func(t *testing.T) {
		countryTest := func(placementDef string, shouldBeIncluded []location.CountryCode, shouldBeExcluded []location.CountryCode) {
			p := TestPlacementDefinitions()
			err := p.AddPlacementFromString("11:" + placementDef)
			require.NoError(t, err)
			filters := p[storj.PlacementConstraint(11)]
			require.NotNil(t, filters)
			for _, code := range shouldBeExcluded {
				require.False(t, filters.Match(&SelectedNode{
					CountryCode: code,
				}), "%s shouldn't be included in placement %s", code, placementDef)
			}
			for _, code := range shouldBeIncluded {
				require.True(t, filters.Match(&SelectedNode{
					CountryCode: code,
				}), "%s is not included in placement %s", code, placementDef)
			}
		}
		countryTest(`country("GB")`, []location.CountryCode{location.UnitedKingdom}, []location.CountryCode{location.Germany, location.UnitedStates})
		countryTest(`country("EU")`, []location.CountryCode{location.Germany, location.Hungary}, []location.CountryCode{location.UnitedStates, location.Norway, location.Iceland})
		countryTest(`country("EEA")`, []location.CountryCode{location.Germany, location.Hungary, location.Norway, location.Iceland}, []location.CountryCode{location.UnitedStates})
		countryTest(`country("ALL","!EU")`, []location.CountryCode{location.Norway, location.India}, []location.CountryCode{location.Germany, location.Hungary})
		countryTest(`country("ALL", "!RU", "!BY")`, []location.CountryCode{location.Norway, location.India, location.UnitedStates}, []location.CountryCode{location.Russia, location.Belarus})
		countryTest(`country("EU", "!DE")`, []location.CountryCode{location.Hungary, location.TheNetherlands}, []location.CountryCode{location.Germany, location.UnitedStates, location.Russia})

	})

	t.Run("tag rule", func(t *testing.T) {
		tagged := func(key string, value string) NodeTags {
			return NodeTags{{
				Signer: signer,
				Name:   key,
				Value:  []byte(value),
			},
			}
		}

		p := TestPlacementDefinitions()
		err := p.AddPlacementFromString(`11:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar")`)
		require.NoError(t, err)
		filters := p[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.True(t, filters.Match(&SelectedNode{
			Tags: tagged("foo", "bar"),
		}))

		testCases := []struct {
			name          string
			placement     string
			includedNodes []*SelectedNode
			excludedNodes []*SelectedNode
		}{
			{
				name:      "simple tag",
				placement: `11:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar")`,
				includedNodes: []*SelectedNode{
					{
						Tags: tagged("foo", "bar"),
					},
				},
				excludedNodes: []*SelectedNode{
					{
						CountryCode: location.Germany,
					},
				},
			},
			{
				name:      "tag not empty",
				placement: `11:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo",notEmpty())`,
				includedNodes: []*SelectedNode{
					{
						Tags: tagged("foo", "barx"),
					},
					{
						Tags: tagged("foo", "bar"),
					},
				},
				excludedNodes: []*SelectedNode{
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
				includedNodes: []*SelectedNode{
					{
						Tags: tagged("foo", ""),
					},
				},
				excludedNodes: []*SelectedNode{
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
				p := TestPlacementDefinitions()
				err := p.AddPlacementFromString(tc.placement)
				require.NoError(t, err)
				filters := p[storj.PlacementConstraint(11)]
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
		p := TestPlacementDefinitions()
		err := p.AddPlacementFromString(`1:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar");2:exclude(placement(1))`)
		require.NoError(t, err)

		require.True(t, p[storj.PlacementConstraint(1)].Match(&SelectedNode{
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))

		placement2 := p[storj.PlacementConstraint(2)]
		require.False(t, placement2.Match(&SelectedNode{
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))
		require.True(t, placement2.Match(&SelectedNode{
			CountryCode: location.Germany,
		}))
	})

	t.Run("placement reuse wrong", func(t *testing.T) {
		p := TestPlacementDefinitions()
		err := p.AddPlacementFromString(`1:exclude(placement(2));2:country("DE")`)
		require.True(t, ErrPlacement.Has(err))
		require.ErrorContains(t, err, "referenced before defined")
	})

	t.Run("all rules", func(t *testing.T) {
		for _, syntax := range []string{
			`11:all(country("GB"),tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar"))`,
			`11:country("GB") && tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar")`,
		} {
			p := TestPlacementDefinitions()
			err := p.AddPlacementFromString(syntax)
			require.NoError(t, err)
			filters := p[storj.PlacementConstraint(11)]
			require.NotNil(t, filters)
			require.True(t, filters.Match(&SelectedNode{
				CountryCode: location.UnitedKingdom,
				Tags: NodeTags{
					{
						Signer: signer,
						Name:   "foo",
						Value:  []byte("bar"),
					},
				},
			}))
			require.False(t, filters.Match(&SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))
			require.False(t, filters.Match(&SelectedNode{
				CountryCode: location.Germany,
				Tags: NodeTags{
					{
						Signer: signer,
						Name:   "foo",
						Value:  []byte("bar"),
					},
				},
			}))
		}
		t.Run("invalid", func(t *testing.T) {
			p := TestPlacementDefinitions()
			err := p.AddPlacementFromString("10:1 && 2")
			require.Error(t, err)
		})
	})

	t.Run("multi rule", func(t *testing.T) {
		p := TestPlacementDefinitions()
		err := p.AddPlacementFromString(`11:country("GB");12:country("DE")`)
		require.NoError(t, err)

		filters := p[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.True(t, filters.Match(&SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.False(t, filters.Match(&SelectedNode{
			CountryCode: location.Germany,
		}))
		require.Equal(t, `country("GB")`, fmt.Sprintf("%s", filters.NodeFilter))

		filters = p[storj.PlacementConstraint(12)]
		require.NotNil(t, filters)
		require.False(t, filters.Match(&SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.True(t, filters.Match(&SelectedNode{
			CountryCode: location.Germany,
		}))

	})

	t.Run("OR", func(t *testing.T) {
		p := TestPlacementDefinitions()
		err := p.AddPlacementFromString(`11:country("GB") || country("DE")`)
		require.NoError(t, err)

		filters := p[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.True(t, filters.Match(&SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.True(t, filters.Match(&SelectedNode{
			CountryCode: location.Germany,
		}))
		require.Equal(t, `(country("GB") || country("DE"))`, fmt.Sprintf("%s", filters.NodeFilter))
	})

	t.Run("OR combined with AND", func(t *testing.T) {
		p := TestPlacementDefinitions()
		err := p.AddPlacementFromString(`11:((country("GB") || country("DE")) && tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar"))`)
		require.NoError(t, err)

		filters := p[storj.PlacementConstraint(11)]
		require.NotNil(t, filters)
		require.False(t, filters.Match(&SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.False(t, filters.Match(&SelectedNode{
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))
		require.True(t, filters.Match(&SelectedNode{
			CountryCode: location.Germany,
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "foo",
					Value:  []byte("bar"),
				},
			},
		}))
		require.Equal(t, `((country("GB") || country("DE")) && tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","foo","bar"))`, fmt.Sprintf("%s", filters.NodeFilter))
	})

	t.Run("annotation usage", func(t *testing.T) {
		t.Run("normal", func(t *testing.T) {
			t.Parallel()
			p := TestPlacementDefinitions()
			err := p.AddPlacementFromString(`11:annotated(country("GB"),annotation("autoExcludeSubnet","off"))`)
			require.NoError(t, err)
			filters := p[storj.PlacementConstraint(11)]
			require.True(t, filters.Match(&SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))

			require.Equal(t, GetAnnotation(filters, "autoExcludeSubnet"), "off")
		})
		t.Run("with &&", func(t *testing.T) {
			t.Parallel()
			p := TestPlacementDefinitions()
			err := p.AddPlacementFromString(`11:country("GB") && annotation("foo","bar") && annotation("bar","foo")`)
			require.NoError(t, err)

			filters := p[storj.PlacementConstraint(11)]
			require.True(t, filters.Match(&SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))
			require.Equal(t, "bar", GetAnnotation(filters, "foo"))
			require.Equal(t, "foo", GetAnnotation(filters, "bar"))
			require.Equal(t, "", GetAnnotation(filters, "kossuth"))
		})
		t.Run("chained", func(t *testing.T) {
			t.Parallel()
			p := TestPlacementDefinitions()
			err := p.AddPlacementFromString(`11:annotated(annotated(country("GB"),annotation("foo","bar")),annotation("bar","foo"))`)
			require.NoError(t, err)
			filters := p[storj.PlacementConstraint(11)]
			require.True(t, filters.Match(&SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))

			require.Equal(t, "bar", GetAnnotation(filters, "foo"))
			require.Equal(t, "foo", GetAnnotation(filters, "bar"))
			require.Equal(t, "", GetAnnotation(filters, "kossuth"))
		})
		t.Run("location", func(t *testing.T) {
			p := TestPlacementDefinitions()
			s := fmt.Sprintf(`11:annotated(annotated(country("GB"),annotation("%s","test-location")),annotation("%s","%s"))`, Location, AutoExcludeSubnet, AutoExcludeSubnetOFF)
			require.NoError(t, p.AddPlacementFromString(s))
			filters := p[storj.PlacementConstraint(11)]
			require.True(t, filters.Match(&SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))

			require.Equal(t, AutoExcludeSubnetOFF, GetAnnotation(filters, AutoExcludeSubnet))
			require.Equal(t, "test-location", GetAnnotation(filters, Location))
		})
	})

	t.Run("exclude", func(t *testing.T) {
		p := TestPlacementDefinitions()
		err := p.AddPlacementFromString(`11:exclude(country("GB"))`)
		require.NoError(t, err)
		filters := p[storj.PlacementConstraint(11)]
		require.False(t, filters.Match(&SelectedNode{
			CountryCode: location.UnitedKingdom,
		}))
		require.True(t, filters.Match(&SelectedNode{
			CountryCode: location.Germany,
		}))
	})

	t.Run("legacy geofencing rules", func(t *testing.T) {
		p := TestPlacementDefinitions()
		p.AddLegacyStaticRules()

		t.Run("nr", func(t *testing.T) {
			nr := p[storj.NR]
			require.True(t, nr.Match(&SelectedNode{
				CountryCode: location.UnitedKingdom,
			}))
			require.False(t, nr.Match(&SelectedNode{
				CountryCode: location.Russia,
			}))
			require.False(t, nr.Match(&SelectedNode{
				CountryCode: 0,
			}))
		})
		t.Run("us", func(t *testing.T) {
			us := p[storj.US]
			require.True(t, us.Match(&SelectedNode{
				CountryCode: location.UnitedStates,
			}))
			require.False(t, us.Match(&SelectedNode{
				CountryCode: location.Germany,
			}))
			require.False(t, us.Match(&SelectedNode{
				CountryCode: 0,
			}))
		})

	})

	t.Run("full example", func(t *testing.T) {
		// this is a realistic configuration, compatible with legacy rules + using one node tag for specific placement

		rules1 := TestPlacementDefinitions()
		err := rules1.AddPlacementFromString(`
						10:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","selected",notEmpty());
						13:tag("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4","datacenter","true");
						11:placement(10) && annotation("autoExcludeSubnet","off") && annotation("location","do-not-use");
						12:placement(10) && annotation("autoExcludeSubnet","off") && country("US") && annotation("location","us-select-1");
						0:exclude(placement(10)) && exclude(placement(13)) && annotation("location","global");
						1:country("EU") && exclude(placement(10)) && exclude(placement(13)) && annotation("location","eu-1");
						2:country("EEA") && exclude(placement(10)) && exclude(placement(13)) && annotation("location","eea-1");
						3:country("US") && exclude(placement(10)) && exclude(placement(13)) && annotation("location","us-1");
						4:country("DE") && exclude(placement(10)) && exclude(placement(13)) && annotation("location","de-1");
						6:country("*","!BY", "!RU", "!NONE") && exclude(placement(10)) && exclude(placement(13)) && annotation("location","custom-1");
						14:placement(13) && annotation("autoExcludeSubnet","off") && annotation("location","global-datacenter");`)
		require.NoError(t, err)

		// for countries, it should be the same as above
		rules2 := TestPlacementDefinitions()
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
		testCountries = append(testCountries, EeaCountriesWithoutEu...)
		testCountries = append(testCountries, EuCountries...)

		// check if old geofencing rules are working as before (and string based config is the same as the code base)
		for _, placement := range []storj.PlacementConstraint{storj.EU, storj.EEA, storj.DE, storj.US, storj.NR} {
			filter1, _ := rules1.CreateFilters(placement)
			filter2, _ := rules2.CreateFilters(placement)
			for _, country := range testCountries {
				result1 := filter1.Match(&SelectedNode{
					CountryCode: country,
				})
				result2 := filter2.Match(&SelectedNode{
					CountryCode: country,
				})
				assert.Equal(t, result1, result2, "default legacy rules do not match string based configuration for placement %d and country %s", placement, country)
			}
		}

		filter1, _ := rules1.CreateFilters(storj.NR)
		filter2, _ := rules2.CreateFilters(storj.NR)
		// make sure that new rules exclude location.None from NR
		assert.False(t, filter1.Match(&SelectedNode{}))
		assert.False(t, filter2.Match(&SelectedNode{}))

		// make sure tagged nodes (even from EU) matches only the special placement
		node := &SelectedNode{
			CountryCode: location.Germany,
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "selected",
					Value:  []byte("true"),
				},
			},
		}

		for _, placement := range []storj.PlacementConstraint{storj.EveryCountry, storj.EU, storj.EEA, storj.DE, storj.US, storj.NR} {
			filter, _ := rules1.CreateFilters(placement)
			assert.False(t, filter.Match(node))
		}
		filter, _ := rules1.CreateFilters(6)
		assert.False(t, filter.Match(node))

		// any value is accepted
		filter, _ = rules1.CreateFilters(11)
		assert.True(t, filter.Match(&SelectedNode{
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "selected",
					Value:  []byte("true,something"),
				},
			},
		}))

		// but not empty
		filter, _ = rules1.CreateFilters(11)
		assert.False(t, filter.Match(&SelectedNode{
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "selected",
					Value:  []byte(""),
				},
			},
		}))

		datacenterNode := &SelectedNode{
			CountryCode: location.UnitedStates,
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "datacenter",
					Value:  []byte("true"),
				},
			},
		}
		for _, placement := range []storj.PlacementConstraint{storj.EveryCountry, storj.EU, storj.EEA, storj.DE, storj.US, storj.NR} {
			filter, _ := rules1.CreateFilters(placement)
			value := filter.Match(datacenterNode)
			assert.False(t, value)
		}

		filter, _ = rules1.CreateFilters(13)
		assert.True(t, filter.Match(&SelectedNode{
			Tags: NodeTags{
				{
					Signer: signer,
					Name:   "datacenter",
					Value:  []byte("true"),
				},
			},
		}))

		// check if annotation present on 11,12, but not on other
		for i := 0; i < 20; i++ {
			filter, _ := rules1.CreateFilters(storj.PlacementConstraint(i))
			subnetDisabled := GetAnnotation(filter, AutoExcludeSubnet) == AutoExcludeSubnetOFF
			if i == 11 || i == 12 || i == 14 {
				require.True(t, subnetDisabled, "Placement filter should be disabled for %d", i)
			} else {
				require.False(t, subnetDisabled, "Placement filter should be enabled for %d", i)
			}
		}

	})
}

func TestStringSerialization(t *testing.T) {
	placements := []string{
		`"10:country("GB")`,
	}
	for _, p := range placements {
		// this flow is very similar to the logic of our flag parsing,
		// where viper first parses the value, but later write it out to a string when viper.AllSettings() is called
		// the string representation should be parseable, and have the same information.

		r := ConfigurablePlacementRule{}
		err := r.Set(p)
		require.NoError(t, err)
		serialized := r.String()

		r2 := ConfigurablePlacementRule{}
		err = r2.Set(serialized)
		require.NoError(t, err)

		require.Equal(t, p, r2.String())

	}
}
