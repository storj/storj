// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

// PartnerInfo contains the name and ID of an Open Source Partner
type PartnerInfo struct {
	ID, Name string
}

// FormattedName returns formatted partner name
func (p *PartnerInfo) FormattedName() string {
	return p.ID + "-" + p.Name
}

// Partners contains a list of partners.
type Partners []PartnerInfo

// LoadPartnerInfos returns our current Open Source Partners.
func LoadPartnerInfos() Partners {
	return []PartnerInfo{
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
