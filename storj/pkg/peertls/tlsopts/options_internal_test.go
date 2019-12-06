// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package tlsopts

import (
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/pkg/peertls"
)

func TestRemoveNils(t *testing.T) {
	e1 := fmt.Errorf("error 1")
	f1 := peertls.PeerCertVerificationFunc(func([][]byte, [][]*x509.Certificate) error { return e1 })
	e2 := fmt.Errorf("error 2")
	f2 := peertls.PeerCertVerificationFunc(func([][]byte, [][]*x509.Certificate) error { return e2 })

	l := removeNils([]peertls.PeerCertVerificationFunc{f1, nil, nil, f2})
	require.Equal(t, len(l), 2)
	require.Equal(t, l[0](nil, nil), e1)
	require.Equal(t, l[1](nil, nil), e2)
}
