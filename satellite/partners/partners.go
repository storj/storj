// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package partners

// Partner contains the name and ID of an Open Source Partner
type Partner struct {
	ID, Name string
}

// Partners contains a list of partners.
type Partners []Partner

// LoadPartners returns our current Open Source Partners.
func LoadPartners() Partners {
	return []Partner{
		{
			Name: "Couchbase",
			ID:   "OSPP001",
		}, {
			Name: "MongoDB",
			ID:   "OSPP002",
		}, {
			Name: "FileZilla",
			ID:   "OSPP003",
		}, {
			Name: "InfluxDB",
			ID:   "OSPP004",
		}, {
			Name: "Kafka",
			ID:   "OSPP005",
		}, {
			Name: "Minio",
			ID:   "OSPP006",
		}, {
			Name: "Nextcloud",
			ID:   "OSPP007",
		}, {
			Name: "MariaDB",
			ID:   "OSPP008",
		}, {
			Name: "Plesk",
			ID:   "OSPP009",
		}, {
			Name: "Pydio",
			ID:   "OSPP010",
		}, {
			Name: "Zenko",
			ID:   "OSPP011",
		},
	}
}
