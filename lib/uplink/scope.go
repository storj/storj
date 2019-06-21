package uplink

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type BucketScope struct {
	Bucket string

	EncryptionAccess EncryptionAccess
}

type Scope struct {
	SatelliteURL string

	APIKey APIKey

	ProjectSecret *string

	Buckets []BucketScope
}

// Unmarshal unmarshals a base58 encoded scope protobuf and decodes
// the fields into the Scope convenience type. It will return an error if the
// protobuf is malformed or field validation fails.
func ParseScope(scopeb58 string) (*Scope, error) {
	data, version, err := base58.CheckDecode(scopeb58)
	if err != nil || version != 0 {
		return nil, errs.New("invalid scope format")
	}

	p := new(pb.Scope)
	if err := proto.Unmarshal(data, p); err != nil {
		return nil, errs.New("unable to unmarshal scope: %v", err)
	}

	if len(p.SatelliteUrl) == 0 {
		return nil, errs.New("scope missing satellite URL")
	}

	apiKey, err := parseRawAPIKey(p.ApiKey)
	if err != nil {
		return nil, errs.New("scope has malformed api key: %v", err)
	}

	var buckets []BucketScope
	for _, b := range p.Buckets {
		switch {
		case len(b.Bucket) == 0:
			return nil, errs.New("bucket scope missing bucket field")
		case len(b.Key) == 0:
			return nil, errs.New("bucket scope missing encryption access key")
		case len(b.Key) != storj.KeySize:
			// Key length validation is necessary because storj.NewKey does not do
			// length checks for backcompat reasons.
			return nil, errs.New("bucket scope encryption access key is of length %d; expected %d", len(b.Key), storj.KeySize)
		}

		key, err := storj.NewKey(b.Key)
		if err != nil {
			return nil, errs.New("scope encryption access key is malformed: %v", err)
		}

		buckets = append(buckets, BucketScope{
			Bucket: string(b.Bucket),
			EncryptionAccess: EncryptionAccess{
				Key:                   *key,
				EncryptedPathPrefix:   string(b.EncryptedPathPrefix),
				UnencryptedPathPrefix: string(b.UnencryptedPathPrefix),
			},
		})
	}

	return &Scope{
		SatelliteURL:  p.SatelliteUrl,
		APIKey:        apiKey,
		ProjectSecret: stringToStringPtr(p.ProjectSecret),
		Buckets:       buckets,
	}, nil
}

func (s *Scope) Serialize() (string, error) {
	switch {
	case len(s.SatelliteURL) == 0:
		return "", errs.New("scope missing satellite URL")
	case s.APIKey.IsZero():
		return "", errs.New("scope missing api key")
	}

	var buckets []*pb.BucketScope
	for _, b := range s.Buckets {
		switch {
		case len(b.Bucket) == 0:
			return "", errs.New("scope missing bucket")
		case b.EncryptionAccess.Key.IsZero():
			return "", errs.New("scope missing encryption access key")
		}

		buckets = append(buckets, &pb.BucketScope{
			Bucket:                []byte(b.Bucket),
			Key:                   b.EncryptionAccess.Key[:],
			EncryptedPathPrefix:   []byte(b.EncryptionAccess.EncryptedPathPrefix),
			UnencryptedPathPrefix: []byte(b.EncryptionAccess.UnencryptedPathPrefix),
		})
	}

	data, err := proto.Marshal(&pb.Scope{
		SatelliteUrl:  s.SatelliteURL,
		ApiKey:        s.APIKey.serializeRaw(),
		ProjectSecret: stringPtrToString(s.ProjectSecret),
		Buckets:       buckets,
	})
	if err != nil {
		return "", errs.New("unable to marshal scope: %v", err)
	}

	return base58.CheckEncode(data, 0), nil
}

func stringPtrToString(sp *string) string {
	if sp != nil {
		return *sp
	}
	return ""
}

func stringToStringPtr(s string) *string {
	if s != "" {
		return &s
	}
	return nil
}
