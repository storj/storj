package transport

import (
	"context"
	"net"
	"strings"

	"github.com/zeebo/errs"
	"google.golang.org/grpc/credentials"

	"storj.io/storj/pkg/peertls"
)

//Credentials wraps credential.Credentials, ensuring invalid
// server certificates don't hang dialing.
type Credentials struct {
	credentials.TransportCredentials
}

// NonTemporaryError is an error with a `Temporary` method which always returns false.
// It is intended for use with grpc.
//
// (see https://godoc.org/google.golang.org/grpc#WithDialer
// and https://godoc.org/google.golang.org/grpc#FailOnNonTempDialError).
type NonTemporaryError struct {
	Err error
}

// ClientHandshake returns a non-temporary error if an error is returned from
// the underlying `ClientHandshake` call.
func (creds *Credentials) ClientHandshake(ctx context.Context, authority string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	tlsConn, authInfo, err := creds.TransportCredentials.ClientHandshake(ctx, authority, conn)
	if err != nil {
		isCertError := peertls.ErrVerifyPeerCert.Has(err) || strings.Contains(err.Error(), "bad certificate")
		if isCertError {
			return tlsConn, authInfo, NewNonTemporaryError(err)
		}
	}
	return tlsConn, authInfo, err
}

// NewNonTemporaryError returns a new temporary error for use with grpc.
func NewNonTemporaryError(err error) NonTemporaryError {
	return NonTemporaryError{
		Err: errs.Wrap(err),
	}
}

// Error implements the error interface
func (nte NonTemporaryError) Error() string {
	return nte.Err.Error()
}

// Temporary returns false to indicate that is is a non-temporary error
func (nte NonTemporaryError) Temporary() bool {
	return false
}
