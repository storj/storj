// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import "storj.io/storj/shared/location"

// EuCountries defines the member countries of European Union.
var EuCountries = []location.CountryCode{
	location.Austria,
	location.Belgium,
	location.Bulgaria,
	location.Croatia,
	location.Cyprus,
	location.Czechia,
	location.Denmark,
	location.Estonia,
	location.Finland,
	location.France,
	location.Germany,
	location.Greece,
	location.Hungary,
	location.Ireland,
	location.Italy,
	location.Lithuania,
	location.Latvia,
	location.Luxembourg,
	location.Malta,
	location.TheNetherlands,
	location.Poland,
	location.Portugal,
	location.Romania,
	location.Slovenia,
	location.Slovakia,
	location.Spain,
	location.Sweden,
}

// EeaCountriesWithoutEu defined the EEA countries.
var EeaCountriesWithoutEu = []location.CountryCode{
	location.Iceland,
	location.Liechtenstein,
	location.Norway,
}
