// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"encoding/base64"
	"encoding/json"
	"path"

	"github.com/zeebo/errs"
)

var (
	// NoMatchPartnerIDErr is the error class used when an offer has reached its redemption capacity
	NoMatchPartnerIDErr = errs.Class("partner not exist")
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
		"120bf202-8252-437e-ac12-0e364bee852e": PartnerInfo{
			Name: "Blocknify",
			ID:   "120bf202-8252-437e-ac12-0e364bee852e",
		},
		"53688ea5-8695-4060-a2c6-b56969217909": PartnerInfo{
			Name: "Breaker",
			ID:   "53688ea5-8695-4060-a2c6-b56969217909",
		},
		"2fb801c6-a6d7-4d82-a838-32fef98cc398": PartnerInfo{
			Name: "Confluent",
			ID:   "2fb801c6-a6d7-4d82-a838-32fef98cc398",
		},
		"e28c8847-b323-4a7d-8111-25a0578a58bb": PartnerInfo{
			Name: "Consensys",
			ID:   "e28c8847-b323-4a7d-8111-25a0578a58bb",
		},
		"0af89ac1-0189-42c6-a47c-e169780b3818": PartnerInfo{
			Name: "Couchbase",
			ID:   "0af89ac1-0189-42c6-a47c-e169780b3818",
		},
		"881b92f6-77aa-42ee-961a-b80009d45dd8": PartnerInfo{
			Name: "Digital Ocean",
			ID:   "881b92f6-77aa-42ee-961a-b80009d45dd8",
		},
		"cadac3fb-6a3f-4d17-9748-cc66d0617d55": PartnerInfo{
			Name: "Deloitte",
			ID:   "cadac3fb-6a3f-4d17-9748-cc66d0617d55",
		},
		"53fb82d7-73ff-4a1a-ab0c-6968cffc850e": PartnerInfo{
			Name: "DVLabs",
			ID:   "53fb82d7-73ff-4a1a-ab0c-6968cffc850e",
		},
		"86c33256-cded-434c-aaac-405343974394": PartnerInfo{
			Name: "Fluree",
			ID:   "86c33256-cded-434c-aaac-405343974394",
		},
		"3e1b911a-c778-47ea-878c-9f3f264f8bc1": PartnerInfo{
			Name: "Flexential",
			ID:   "3e1b911a-c778-47ea-878c-9f3f264f8bc1",
		},
		"706011f3-400e-45eb-a796-90cce2a7d67e": PartnerInfo{
			Name: "Heroku",
			ID:   "706011f3-400e-45eb-a796-90cce2a7d67e",
		},
		"1519bdee-ed18-45fe-86c6-4c7fa9668a14": PartnerInfo{
			Name: "Infura",
			ID:   "1519bdee-ed18-45fe-86c6-4c7fa9668a14",
		},
		"e56c6a65-d5bf-457a-a414-e55c36624f73": PartnerInfo{
			Name: "GroundX",
			ID:   "e56c6a65-d5bf-457a-a414-e55c36624f73",
		},
		"8ee019ef-2aae-4867-9c18-41c65ea318c4": PartnerInfo{
			Name: "MariaDB",
			ID:   "8ee019ef-2aae-4867-9c18-41c65ea318c4",
		},
		"3405a882-0cb2-4f91-a6e0-21be193b80e5": PartnerInfo{
			Name: "Netki",
			ID:   "3405a882-0cb2-4f91-a6e0-21be193b80e5",
		},
		"a1ba07a4-e095-4a43-914c-1d56c9ff5afd": PartnerInfo{
			Name: "FileZilla",
			ID:   "a1ba07a4-e095-4a43-914c-1d56c9ff5afd",
		},
		"e50a17b3-4d82-4da7-8719-09312a83685d": PartnerInfo{
			Name: "InfluxDB",
			ID:   "e50a17b3-4d82-4da7-8719-09312a83685d",
		},
		"c10228c2-af70-4e4d-be49-e8bfbe9ca8ef": PartnerInfo{
			Name: "Mysterium Network",
			ID:   "c10228c2-af70-4e4d-be49-e8bfbe9ca8ef",
		},
		"OSPP005": PartnerInfo{
			Name: "Kafka",
			ID:   "OSPP005",
		},
		"5bffe844-5da7-4aa9-bf37-7d695cf819f2": PartnerInfo{
			Name: "Minio",
			ID:   "5bffe844-5da7-4aa9-bf37-7d695cf819f2",
		},
		"42f588fb-f39d-4886-81af-b614ca16ce37": PartnerInfo{
			Name: "Nextcloud",
			ID:   "42f588fb-f39d-4886-81af-b614ca16ce37",
		},
		"3b53a9b3-2005-476c-9ffd-894ed832abe4": PartnerInfo{
			Name: "Node Haven",
			ID:   "3b53a9b3-2005-476c-9ffd-894ed832abe4",
		},
		"dc01ed96-2990-4819-9cb3-45d4846b9ad1": PartnerInfo{
			Name: "Plesk",
			ID:   "dc01ed96-2990-4819-9cb3-45d4846b9ad1",
		},
		"b02b9f0d-fac7-439c-8ba2-0c4634d5826f": PartnerInfo{
			Name: "Pydio",
			ID:   "b02b9f0d-fac7-439c-8ba2-0c4634d5826f",
		},
		"57855387-5a58-4a2b-97d2-15b1d76eea3c": PartnerInfo{
			Name: "Raiden Network",
			ID:   "57855387-5a58-4a2b-97d2-15b1d76eea3c",
		},
		"4400d796-3777-4964-8536-22a4ae439ed3": PartnerInfo{
			Name: "Satoshi Soup",
			ID:   "4400d796-3777-4964-8536-22a4ae439ed3",
		},
		"6e40f882-ef77-4a5d-b5ad-18525d3df023": PartnerInfo{
			Name: "Sirin Labs",
			ID:   "6e40f882-ef77-4a5d-b5ad-18525d3df023",
		},
		"b6114126-c06d-49f9-8d23-3e0dd2e350ab": PartnerInfo{
			Name: "Status Messenger",
			ID:   "b6114126-c06d-49f9-8d23-3e0dd2e350ab",
		},
		"aeedbe32-1519-4320-b2f4-33725c65af54": PartnerInfo{
			Name: "Temporal",
			ID:   "aeedbe32-1519-4320-b2f4-33725c65af54",
		},
		"7bf23e53-6393-4bd0-8bf9-53ecf0de742f": PartnerInfo{
			Name: "Terminal.co",
			ID:   "7bf23e53-6393-4bd0-8bf9-53ecf0de742f",
		},
		"8cd605fa-ad00-45b6-823e-550eddc611d6": PartnerInfo{
			Name: "Zenko",
			ID:   "8cd605fa-ad00-45b6-823e-550eddc611d6",
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
