// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"encoding/base64"
	"encoding/json"
)

// PartnerInfo contains the name and ID of an Open Source Partner
type PartnerInfo struct {
	ID, Name string
}

// Partners contains a list of partners.
type Partners map[string]PartnerInfo

// LoadPartnerInfos returns our current Open Source Partners.
func LoadPartnerInfos() Partners {
	//TODO: implement actual function call to get partner infos
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

// GeneratePartnerLink returns base64 encoded partner referral link
func GeneratePartnerLink(offerName string) ([]string, error) {
	referralInfo := &referralInfo{UserID: "", PartnerID: GetPartnerIDByName(offerName)}
	refJSON, err := json.Marshal(referralInfo)
	if err != nil {
		return nil, err
	}

	domians := getTardigradeDomains()
	referralLinks := make([]string, len(domians))

	for i, url := range domians {
		referralLinks[i] = url + base64.StdEncoding.EncodeToString(refJSON)
	}

	return referralLinks, nil
}

// GetPartnerIDByName returns a partner id based on its name
func GetPartnerIDByName(name string) string {
	partner, ok := LoadPartnerInfos()[name]
	if !ok {
		return ""
	}
	return partner.ID
}
