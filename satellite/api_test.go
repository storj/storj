// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package satellite_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite"
)

const certExample = `
-----BEGIN CERTIFICATE-----
MIIBYjCCAQegAwIBAgIQILox8KzJildxdqYqBIdPYTAKBggqhkjOPQQDAjAQMQ4w
DAYDVQQKEwVTdG9yajAiGA8wMDAxMDEwMTAwMDAwMFoYDzAwMDEwMTAxMDAwMDAw
WjAQMQ4wDAYDVQQKEwVTdG9yajBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABKrU
7UPNIbn7Ll6XTRlLTJF+CBmssEHC9EJDCE5nFUUdPJ7XTlwY5pIAHthW2SN1wIF7
riWBVOqZMhW/7nxy8NmjPzA9MA4GA1UdDwEB/wQEAwIFoDAdBgNVHSUEFjAUBggr
BgEFBQcDAQYIKwYBBQUHAwIwDAYDVR0TAQH/BAIwADAKBggqhkjOPQQDAgNJADBG
AiEAjuL/DcSXUp1WMJXb0YfDG3choj5efMfq2EeB7MnsTR0CIQDOtGUIvTjB+CzJ
wyxU+oxqT796GNVeYMyearuQ1QZCJw==
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBcDCCARWgAwIBAgIQVJK5KKSuzc5UYE/Mz0F1AjAKBggqhkjOPQQDAjAQMQ4w
DAYDVQQKEwVTdG9yajAiGA8wMDAxMDEwMTAwMDAwMFoYDzAwMDEwMTAxMDAwMDAw
WjAQMQ4wDAYDVQQKEwVTdG9yajBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABKz1
EFtvbYmWiJlVu2r6UyZKMFiczHWmpUhPU2vPcVFPQ+hLsbDRa8dFTzmoqQ5HWAov
RL64plhFJDUR2lYZvKujTTBLMA4GA1UdDwEB/wQEAwICBDAPBgNVHRMBAf8EBTAD
AQH/MB0GA1UdDgQWBBQ9Qt80xie1vPyCQ4RU+nFnjQe1djAJBgSINwIBBAEAMAoG
CCqGSM49BAMCA0kAMEYCIQCkvgnd51j1QEdvtIEQGZGHGAE3n7a7bhJ1MK+WBaiu
LgIhALqIm7uONq0H1xQEtV2nnGFUHC6v+6tIOu2Z7W92DSzb
-----END CERTIFICATE-----
`

// generated with 'tag-signer sign --node-id 1AujhDBDBhfYUhEH8cpFMymPu5yLYkQBCfFYbdSN5AVL5Bu6W9 --identity-dir `pwd` foo=bar' using the key of the cert, above.
const signedTag = "CqIBCjQKIBaACbi52g7o8pQaX+hkw6TJNDHJ0UVqSLQtibPHoqkAEgoKA2ZvbxIDYmFyGKWoiaYGGiAJ20r8flrnkM9YBpKEvXmeXhYLfwx/HUEwu9Lv4QzaACJIMEYCIQCHk1wkMPYB6ZkmGGSU9M2p0E+WdjXCzxy4Kx2+gUP9wgIhALvcaTIgHCFOsc9gjGhSA0wS8L+cXaXQQPvq2nFapiLc"

func TestLoadAuthorities(t *testing.T) {
	selfIdentity := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())
	t.Run("real cert", func(t *testing.T) {
		certPath := filepath.Join(t.TempDir(), "identity.cert")
		err := os.WriteFile(certPath, []byte(certExample), 0644)
		require.NoError(t, err)

		a, err := satellite.LoadAuthorities(selfIdentity.PeerIdentity(), certPath+","+certPath)
		require.NoError(t, err)

		decoded, err := base64.StdEncoding.DecodeString(signedTag)
		require.NoError(t, err)

		tags := &pb.SignedNodeTagSets{}
		err = pb.Unmarshal(decoded, tags)
		require.NoError(t, err)

		require.Len(t, tags.Tags, 1)
		_, err = a.Verify(testcontext.New(t), tags.Tags[0])
		require.NoError(t, err)

	})
	t.Run("invalid file", func(t *testing.T) {
		_, err := satellite.LoadAuthorities(selfIdentity.PeerIdentity(), "none")
		require.Error(t, err)
	})

}
