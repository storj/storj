// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import "github.com/zeebo/errs"

var (
	// NoMatchPartnerIDErr is the error class used when an offer has reached its redemption capacity
	NoMatchPartnerIDErr = errs.Class("partner not exist")
)

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
		"OSPP001": PartnerInfo{
			Name: "Couchbase",
			ID:   "OSPP001",
		},
		"OSPP002": PartnerInfo{
			Name: "MongoDB",
			ID:   "OSPP002",
		},
		"OSPP003": PartnerInfo{
			Name: "FileZilla",
			ID:   "OSPP003",
		},
		"OSPP004": PartnerInfo{
			Name: "InfluxDB",
			ID:   "OSPP004",
		},
		"OSPP005": PartnerInfo{
			Name: "Kafka",
			ID:   "OSPP005",
		},
		"OSPP006": PartnerInfo{
			Name: "Minio",
			ID:   "OSPP006",
		},
		"OSPP007": PartnerInfo{
			Name: "Nextcloud",
			ID:   "OSPP007",
		},
		"OSPP008": PartnerInfo{
			Name: "MariaDB",
			ID:   "OSPP008",
		},
		"OSPP009": PartnerInfo{
			Name: "Plesk",
			ID:   "OSPP009",
		},
		"OSPP010": PartnerInfo{
			Name: "Pydio",
			ID:   "OSPP010",
		},
		"OSPP011": PartnerInfo{
			Name: "Zenko",
			ID:   "OSPP011",
		},
	}
}

// GetPartnerID returns partner ID based on partner name
func GetPartnerID(partnerName string) (partnerID string, err error) {
	partners := LoadPartnerInfos()
	for i := range partners {
		if partners[i].Name == partnerName {
			return partners[i].ID, nil
		}
	}

	return "", errs.New("partner id not found")
}
