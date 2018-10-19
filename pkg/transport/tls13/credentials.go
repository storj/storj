package tls13

import (
	"context"
	"net"
	"strings"

	"github.com/bifurcation/mint"
	"google.golang.org/grpc/credentials"
)

// Credentials is the credentials required for authenticating a connection using mint.TLS.
type Credentials struct {
	// TLS configuration
	Config *mint.Config
}

// NewCredentials uses c to construct a TransportCredentials based on TLS.
func NewCredentials(c *mint.Config) credentials.TransportCredentials {
	tc := &Credentials{c.Clone()}
	tc.Config.NextProtos = []string{"h2"}
	return tc
}

// ClientHandshake does the authentication handshake specified by the corresponding
// authentication protocol on rawConn for clients. It returns the authenticated
// connection and the corresponding auth information about the connection.
// Implementations must use the provided context to implement timely cancellation.
// gRPC will try to reconnect if the error returned is a temporary error
// (io.EOF, context.DeadlineExceeded or err.Temporary() == true).
// If the returned error is a wrapper error, implementations should make sure that
// the error implements Temporary() to have the correct retry behaviors.
//
// If the returned net.Conn is closed, it MUST close the net.Conn provided.
func (c *Credentials) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	// use local cfg to avoid clobbering ServerName if using multiple endpoints
	cfg := c.Config.Clone()
	if cfg.ServerName == "" {
		colonPos := strings.LastIndex(authority, ":")
		if colonPos == -1 {
			colonPos = len(authority)
		}
		cfg.ServerName = authority[:colonPos]
	}
	conn := mint.Client(rawConn, cfg)
	errChannel := make(chan error, 1)
	go func() {
		errChannel <- conn.Handshake()
	}()
	select {
	case err := <-errChannel:
		if err != nil && err != mint.AlertNoAlert {
			return nil, nil, err
		}
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	return conn, TLSInfo{conn.ConnectionState()}, nil
}

// ServerHandshake does the authentication handshake for servers. It returns
// the authenticated connection and the corresponding auth information about
// the connection.
//
// If the returned net.Conn is closed, it MUST close the net.Conn provided.
func (c *Credentials) ServerHandshake(rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	conn := mint.Server(rawConn, c.Config)
	if err := conn.Handshake(); err != mint.AlertNoAlert {
		return nil, nil, err
	}
	return conn, TLSInfo{conn.ConnectionState()}, nil
}

// Info provides the ProtocolInfo of this TransportCredentials.
func (c Credentials) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{
		SecurityProtocol: "tls",
		SecurityVersion:  "1.3",
		ServerName:       "storj", //TODO:
	}
}

// Clone makes a copy of this TransportCredentials.
func (c *Credentials) Clone() credentials.TransportCredentials {
	return NewCredentials(c.Config)
}

// OverrideServerName overrides the server name used to verify the hostname on the returned certificates from the server.
// gRPC internals also use it to override the virtual hosting name if it is set.
// It must be called before dialing. Currently, this is only used by grpclb.
func (c *Credentials) OverrideServerName(serverNameOverride string) error {
	c.Config.ServerName = serverNameOverride
	return nil
}

// TLSInfo contains the auth information for a TLS authenticated connection.
// It implements the AuthInfo interface.
type TLSInfo struct {
	State mint.ConnectionState
}

// AuthType returns the type of TLSInfo as a string.
func (t TLSInfo) AuthType() string {
	return "tls"
}

// GetChannelzSecurityValue returns security info requested by channelz.
func (t TLSInfo) GetChannelzSecurityValue() credentials.ChannelzSecurityValue {
	v := &credentials.TLSChannelzSecurityValue{
		StandardName: cipherSuiteLookup[t.State.CipherSuite.Suite],
	}
	// Currently there's no way to get LocalCertificate info from tls package.
	if len(t.State.PeerCertificates) > 0 {
		v.RemoteCertificate = t.State.PeerCertificates[0].Raw
	}
	return v
}

var cipherSuiteLookup = map[mint.CipherSuite]string{
	mint.TLS_AES_128_GCM_SHA256:       "TLS_AES_128_GCM_SHA256",
	mint.TLS_AES_256_GCM_SHA384:       "TLS_AES_256_GCM_SHA384",
	mint.TLS_CHACHA20_POLY1305_SHA256: "TLS_CHACHA20_POLY1305_SHA256",
	mint.TLS_AES_128_CCM_SHA256:       "TLS_AES_128_CCM_SHA256",
	mint.TLS_AES_256_CCM_8_SHA256:     "TLS_AES_256_CCM_8_SHA256",
}
