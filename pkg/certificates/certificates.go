// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
)

const (
	// AuthorizationsBucket is the bucket used with a bolt-backed authorizations DB.
	AuthorizationsBucket = "authorizations"
	tokenDataLength      = 64 // 2^(64*8) =~ 1.34E+154
	tokenDelimiter       = ":"
	tokenVersion         = 0
)

var (
	mon = monkit.Package()
	// ErrAuthorization is used when an error occurs involving an authorization.
	ErrAuthorization = errs.Class("authorization error")
	// ErrAuthorizationDB is used when an error occurs involving the authorization database.
	ErrAuthorizationDB = errs.Class("authorization db error")
	// ErrToken is used when a token is invalid
	ErrToken = errs.Class("token error")
	// ErrAuthorizationCount is used when attempting to create an invalid number of authorizations.
	ErrAuthorizationCount = ErrAuthorizationDB.New("cannot add less than one authorizations")
	// ErrTokenDelimiter is used when the delimiter is missing from a token string
	ErrTokenDelimiter = ErrToken.New("delimiter missing")
	// ErrTokenUserID is used when the userID is missing from a token string
	ErrTokenUserID = ErrToken.New("user ID missing")
	// ErrTokenData is used when the token data is the wrong length
	ErrTokenData = ErrToken.New("data size mismatch")
)

// CertSignerConfig is a config struct for use with a certificate signing service
type CertSignerConfig struct {
	Overwrite          bool   `help:"if true, overwrites config AND authorization db is truncated" default:"false"`
	AuthorizationDBURL string `help:"url to the certificate signing authorization database" default:"bolt://$CONFDIR/authorizations.db"`
}

// CertificateSigner implements pb.CertificatesServer
type CertificateSigner struct {
	Log *zap.Logger
}

// AuthorizationDB stores authorizations which may be claimed in exchange for a
// certificate signature.
type AuthorizationDB struct {
	DB storage.KeyValueStore
}

// Authorizations is a slice of authorizations for convenient de/serialization
// and grouping.
type Authorizations []*Authorization

// Authorization represents a single-use authorization token and its status
type Authorization struct {
	Token Token
	Claim *Claim
}

// Token is a random byte array to be used like a pre-shared key for claiming
// certificate signatures.
type Token struct {
	// NB: currently email address for convenience
	UserID string
	Data   [tokenDataLength]byte
}

// Claim holds information about the circumstances under which an authorization
// token was claimed.
type Claim struct {
	IP         string
	Timestamp  int64
	Identity   *provider.PeerIdentity
	SignedCert *x509.Certificate
}

// NewServer creates a new certificate signing grpc server
func NewServer(log *zap.Logger) pb.CertificatesServer {
	srv := CertificateSigner{
		Log: log,
	}

	return &srv
}

// NewAuthorization creates a new, unclaimed authorization with a random token value
func NewAuthorization(userID string) (*Authorization, error) {
	token := Token{UserID: userID}
	_, err := rand.Read(token.Data[:])
	if err != nil {
		return nil, ErrAuthorization.Wrap(err)
	}

	return &Authorization{
		Token: token,
	}, nil
}

// ParseToken splits the token string on the delimiter to get a userID and data
// for a token and base58 decodes the data.
func ParseToken(tokenString string) (*Token, error) {
	splitAt := strings.LastIndex(tokenString, tokenDelimiter)
	if splitAt == -1 {
		return nil, ErrTokenDelimiter
	}

	userID, b58Data := tokenString[:splitAt], tokenString[splitAt+1:]
	if len(userID) == 0 {
		return nil, ErrTokenUserID
	}

	data, _, err := base58.CheckDecode(b58Data)
	if err != nil {
		return nil, ErrToken.Wrap(err)
	}

	if len(data) != tokenDataLength {
		return nil, ErrTokenData
	}
	t := &Token{
		UserID: userID,
		Data:   [tokenDataLength]byte{},
	}
	copy(t.Data[:], data)
	return t, nil
}

// NewAuthDB creates or opens the authorization database specified by the config
func (c CertSignerConfig) NewAuthDB() (*AuthorizationDB, error) {
	// TODO: refactor db selection logic?
	driver, source, err := utils.SplitDBURL(c.AuthorizationDBURL)
	if err != nil {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}

	authDB := new(AuthorizationDB)
	switch driver {
	case "bolt":
		if c.Overwrite {
			if err := os.Remove(source); err != nil {
				return nil, err
			}
		}

		authDB.DB, err = boltdb.New(source, AuthorizationsBucket)
		if err != nil {
			return nil, ErrAuthorizationDB.Wrap(err)
		}
	case "redis":
		redisClient, err := redis.NewClientFrom(c.AuthorizationDBURL)
		if err != nil {
			return nil, ErrAuthorizationDB.Wrap(err)
		}

		if c.Overwrite {
			if err := redisClient.FlushDB(); err != nil {
				return nil, err
			}
		}

		authDB.DB = redisClient
	default:
		return nil, ErrAuthorizationDB.New("database scheme not supported: %s", driver)
	}

	return authDB, nil
}

// Run implements the responsibility interface, starting a certificate signing server.
func (c CertSignerConfig) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	srv := NewServer(zap.L())
	pb.RegisterCertificatesServer(server.GRPC(), srv)

	return server.Run(ctx)
}

// Sign signs a valid certificate signing request's cert.
func (c CertificateSigner) Sign(ctx context.Context, req *pb.SigningRequest) (*pb.SigningResponse, error) {
	// lookup authtoken
	// sign cert
	// send response
	return &pb.SigningResponse{}, nil
}

// Close closes the authorization database's underlying store.
func (a *AuthorizationDB) Close() error {
	return ErrAuthorizationDB.Wrap(a.DB.Close())
}

// Create creates a new authorization and adds it to the authorization database.
func (a *AuthorizationDB) Create(userID string, count int) (Authorizations, error) {
	if len(userID) == 0 {
		return nil, ErrAuthorizationDB.New("userID cannot be empty")
	}
	if count < 1 {
		return nil, ErrAuthorizationCount
	}

	existingAuths, err := a.Get(userID)
	if err != nil {
		return nil, err
	}

	var (
		newAuths Authorizations
		authErrs utils.ErrorGroup
	)
	for i := 0; i < count; i++ {
		auth, err := NewAuthorization(userID)
		if err != nil {
			authErrs.Add(err)
			continue
		}
		newAuths = append(newAuths, auth)
	}
	if err := authErrs.Finish(); err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}

	existingAuths = append(existingAuths, newAuths...)
	authsBytes, err := existingAuths.Marshal()
	if err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}

	if err := a.DB.Put(storage.Key(userID), authsBytes); err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}

	return newAuths, nil
}

// Get retrieves authorizations by email.
func (a *AuthorizationDB) Get(email string) (Authorizations, error) {
	authsBytes, err := a.DB.Get(storage.Key(email))
	if err != nil && !storage.ErrKeyNotFound.Has(err) {
		return nil, ErrAuthorizationDB.Wrap(err)
	}
	if authsBytes == nil {
		return nil, nil
	}

	var auths Authorizations
	if err := auths.Unmarshal(authsBytes); err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}
	return auths, nil
}

// UserIDs returns a list of all emails present in the authorization database.
func (a *AuthorizationDB) UserIDs() ([]string, error) {
	keys, err := a.DB.List([]byte{}, 0)
	if err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}
	return keys.Strings(), nil
}

// Unmarshal deserializes a set of authorizations
func (a *Authorizations) Unmarshal(data []byte) error {
	decoder := gob.NewDecoder(bytes.NewBuffer(data))
	if err := decoder.Decode(a); err != nil {
		return ErrAuthorization.Wrap(err)
	}
	return nil
}

// Marshal serializes a set of authorizations
func (a Authorizations) Marshal() ([]byte, error) {
	data := new(bytes.Buffer)
	encoder := gob.NewEncoder(data)
	err := encoder.Encode(a)
	if err != nil {
		return nil, ErrAuthorization.Wrap(err)
	}

	return data.Bytes(), nil
}

// Group separates a set of authorizations into a set of claimed and a set of open authorizations.
func (a Authorizations) Group() (claimed, open Authorizations) {
	for _, auth := range a {
		if auth.Claim != nil {
			// TODO: check if claim is valid? what if not?
			claimed = append(claimed, auth)
		} else {
			open = append(open, auth)
		}
	}
	return claimed, open
}

// String implements the stringer interface and prevents authorization data
// from completely leaking into logs.
func (a Authorization) String() string {
	return fmt.Sprintf("%.5s..", a.Token.String())
}

// String implements the stringer interface. Base68 w/ version and checksum bytes
// are used for easy and reliable human transport.
func (t *Token) String() string {
	return fmt.Sprintf("%s:%s", t.UserID, base58.CheckEncode(t.Data[:], tokenVersion))
}
