package certificates

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/gob"
	"fmt"

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
	AuthorizationsBucket = "authorizations"
	tokenLength = 64 // 2^(64*8) =~ 1.34E+154
)

var (
	ErrAuthorization = errs.Class("authorization error")
	ErrNotEnoughRandom = ErrAuthorization.New("unable to read enough random bytes")
	ErrAuthorizationCount = ErrAuthorizationDB.New("cannot add less than one authorizations")
)

type CertSignerConfig struct {
	AuthorizationDBURL string `help:"url to the certificate signing authorization database" default:"bolt://$CONFDIR/authorizations.db"`
}

type CertificateSigner struct {
	Log *zap.Logger
}

type AuthorizationDB struct {
	DB storage.KeyValueStore
}

type Authorizations []*Authorization

type Authorization struct {
	Token Token
	Claim *Claim
}

type Token [tokenLength]byte

type Claim struct {
	IP string
	Timestamp int64
	Identity *provider.PeerIdentity
	SignedCert *x509.Certificate
}

var (
	mon = monkit.Package()
	ErrAuthorizationDB = errs.Class("authorization db error")
)

func NewServer(log *zap.Logger) pb.CertificatesServer {
	srv := CertificateSigner{
		Log: log,
	}

	return &srv
}

func NewAuthorization() (*Authorization, error) {
	var token Token
	i, err := rand.Read(token[:])
	if err != nil {
		return nil, ErrAuthorization.Wrap(err)
	}
	if i != tokenLength {
		return nil, ErrNotEnoughRandom
	}

	return &Authorization{
		Token: token,
	}, nil
}

func (c CertSignerConfig) NewAuthDB() (*AuthorizationDB, error) {
	// TODO: refactor db selection logic?
	driver, source, err := utils.SplitDBURL(c.AuthorizationDBURL)
	if err != nil {
		return nil, peertls.ErrRevocationDB.Wrap(err)
	}

	authDB := new(AuthorizationDB)
	switch driver {
	case "bolt":
		authDB.DB, err = boltdb.New(source, AuthorizationsBucket)
		if err != nil {
			return nil, ErrAuthorizationDB.Wrap(err)
		}
	case "redis":
		authDB.DB, err = redis.NewClientFrom(c.AuthorizationDBURL)
		if err != nil {
			return nil, ErrAuthorizationDB.Wrap(err)
		}
	default:
		return nil, ErrAuthorizationDB.New("database scheme not supported: %s", driver)
	}

	return authDB, nil
}

func (c CertSignerConfig) Run(ctx context.Context, server *provider.Provider) (err error) {
	defer mon.Task()(&ctx)(&err)

	srv := NewServer(zap.L())
	pb.RegisterCertificatesServer(server.GRPC(), srv)

	return server.Run(ctx)
}

func (c CertificateSigner) Sign(ctx context.Context, req *pb.SigningRequest) (*pb.SigningResponse, error) {
	// lookup authtoken
	// sign cert
	// send response
	return &pb.SigningResponse{}, nil
}

func (a *AuthorizationDB) Close() error {
	return ErrAuthorizationDB.Wrap(a.DB.Close())
}

func (a *AuthorizationDB) Create(email string, count int) (Authorizations, error) {
	if count < 1 {
		return nil, ErrAuthorizationCount
	}

	existingAuths, err := a.Get(email)
	if err != nil {
		return nil, err
	}

	var (
		newAuths Authorizations
		authErrs      utils.ErrorGroup
	)
	for i := 0; i < count; i++ {
		auth, err := NewAuthorization()
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

	if err := a.DB.Put(storage.Key(email), authsBytes); err != nil {
		return nil, ErrAuthorizationDB.Wrap(err)
	}

	return newAuths, nil
}

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

func (a *Authorizations) Unmarshal(data []byte) error {
	decoder := gob.NewDecoder(bytes.NewBuffer(data))
	if err := decoder.Decode(a); err != nil {
		return ErrAuthorization.Wrap(err)
	}
	return nil
}

func (a Authorizations) Marshal() ([]byte, error) {
	data := new(bytes.Buffer)
	encoder := gob.NewEncoder(data)
	err := encoder.Encode(a)
	if err != nil {
		return nil, ErrAuthorization.Wrap(err)
	}

	return data.Bytes(), nil
}

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

func (t *Token) String() string {
	return base58.CheckEncode(t[:], 0)
}