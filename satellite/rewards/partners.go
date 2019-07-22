// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

// PartnerInfo contains the name and ID of an Open Source Partner
type PartnerInfo struct {
	ID, Name string
}

// FormattedName returns formatted partner name
func (p PartnerInfo) FormattedName() string {
	return p.ID + "-" + p.Name
}

// Partners contains a list of partners.
type Partners map[string]PartnerInfo

// LoadPartnerInfos returns our current Open Source Partners.
func LoadPartnerInfos() Partners {
	return Partners{
		"Couchbase": PartnerInfo{
			Name: "Couchbase",
			ID:   "OSPP001",
		},
		"MongoDB": PartnerInfo{
			Name: "MongoDB",
			ID:   "OSPP002",
		},
		"FileZilla": PartnerInfo{
			Name: "FileZilla",
			ID:   "OSPP003",
		},
		"InfluxDB": PartnerInfo{
			Name: "InfluxDB",
			ID:   "OSPP004",
		},
		"Kafka": PartnerInfo{
			Name: "Kafka",
			ID:   "OSPP005",
		},
		"Minio": PartnerInfo{
			Name: "Minio",
			ID:   "OSPP006",
		},
		"Nextcloud": PartnerInfo{
			Name: "Nextcloud",
			ID:   "OSPP007",
		},
		"MariaDB": PartnerInfo{
			Name: "MariaDB",
			ID:   "OSPP008",
		},
		"Plesk": PartnerInfo{
			Name: "Plesk",
			ID:   "OSPP009",
		},
		"Pydio": PartnerInfo{
			Name: "Pydio",
			ID:   "OSPP010",
		},
		"Zenko": PartnerInfo{
			Name: "Zenko",
			ID:   "OSPP011",
		},
	}
}
