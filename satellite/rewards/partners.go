// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"encoding/base64"
	"encoding/json"
	"path"

	"github.com/zeebo/errs"
)

// PartnerInfo contains the name and ID of an Open Source Partner
type PartnerInfo struct {
	ID, Name string
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

// GeneratePartnerLink returns base64 encoded partner referral link
func GeneratePartnerLink(offerName string) ([]string, error) {
	pID, err := GetPartnerID(offerName)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	referralInfo := &referralInfo{UserID: "", PartnerID: pID}
	refJSON, err := json.Marshal(referralInfo)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	domains := getTardigradeDomains()
	referralLinks := make([]string, len(domains))
	encoded := base64.StdEncoding.EncodeToString(refJSON)

	for i, url := range domains {
		referralLinks[i] = path.Join(url, encoded)
	}

	return referralLinks, nil
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
