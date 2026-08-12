// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/satellite/geoip"
)

var (
	testInvoicesHeader = strings.Join([]string{
		"period",
		"node-id",
		"node-created-at",
		"node-disqualified",
		"node-gracefulexit",
		"node-wallet",
		"node-wallet-features",
		"node-address",
		"node-last-ip",
		"codes",
		"usage-at-rest",
		"usage-get",
		"usage-put",
		"usage-get-repair",
		"usage-put-repair",
		"usage-get-audit",
		"comp-at-rest",
		"comp-get",
		"comp-put",
		"comp-get-repair",
		"comp-put-repair",
		"comp-get-audit",
		"surge-percent",
		"owed",
		"held",
		"disposed",
		"total-held",
		"total-disposed",
		"total-paid",
		"total-distributed",
		"voluntary-discount",
	}, ",")
	testPaystubsHeader = strings.Join([]string{
		"period",
		"node-id",
		"codes",
		"usage-at-rest",
		"usage-get",
		"usage-put",
		"usage-get-repair",
		"usage-put-repair",
		"usage-get-audit",
		"comp-at-rest",
		"comp-get",
		"comp-put",
		"comp-get-repair",
		"comp-put-repair",
		"comp-get-audit",
		"surge-percent",
		"owed",
		"held",
		"disposed",
		"paid",
		"possibly-distributed",
	}, ",")
	testPrePayoutsHeader = "address,amount,address-kind,mandatory,sanctioned"
)

func TestPrepare(t *testing.T) {
	for _, tt := range []struct {
		name           string
		headerOverride string
		invoicesIn     string
		paystubsOut    string
		payoutsOut     string
		geoIPDBs       []*geoip.MaxmindDB
		skipOFAC       bool
		err            string
	}{
		{
			name: "no invoices",
		},
		{
			name:           "duplicate header",
			headerOverride: "node-id,node-id",
			err:            `strictcsv: CSV header "node-id" is duplicated`,
		},
		{
			name:           "unmapped mapped",
			headerOverride: "JUNKOLA",
			err:            `strictcsv: CSV header "JUNKOLA" is not mapped to struct field`,
		},
		{
			name:           "missing headers",
			headerOverride: "period",
			err: `strictcsv: field headers [` +
				`"codes" ` +
				`"comp-at-rest" ` +
				`"comp-get" ` +
				`"comp-get-audit" ` +
				`"comp-get-repair" ` +
				`"comp-put" ` +
				`"comp-put-repair" ` +
				`"disposed" ` +
				`"held" ` +
				`"node-address" ` +
				`"node-created-at" ` +
				`"node-disqualified" ` +
				`"node-gracefulexit" ` +
				`"node-id" ` +
				`"node-last-ip" ` +
				`"node-wallet" ` +
				`"node-wallet-features" ` +
				`"owed" ` +
				`"surge-percent" ` +
				`"total-disposed" ` +
				`"total-distributed" ` +
				`"total-held" ` +
				`"total-paid" ` +
				`"usage-at-rest" ` +
				`"usage-get" ` +
				`"usage-get-audit" ` +
				`"usage-get-repair" ` +
				`"usage-put" ` +
				`"usage-put-repair" ` +
				`"voluntary-discount"` +
				`] missing from CSV`,
		},
		{
			name: "",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			invoicesHeader := tt.headerOverride
			if invoicesHeader == "" {
				invoicesHeader = testInvoicesHeader
			}

			invoicesIn := strings.NewReader(invoicesHeader + "\n" + tt.invoicesIn)
			paystubsOut := new(bytes.Buffer)
			payoutsOut := new(bytes.Buffer)

			err := Prepare(invoicesIn, paystubsOut, payoutsOut, PrepareConfig{
				GeoIPDBs: tt.geoIPDBs,
				SkipOFAC: tt.skipOFAC,
			})
			if tt.err != "" {
				require.EqualError(t, err, tt.err)
				return
			}
			require.NoError(t, err)

			require.Equal(t, testPaystubsHeader+"\n"+tt.paystubsOut, paystubsOut.String())
			require.Equal(t, testPrePayoutsHeader+"\n"+tt.payoutsOut, payoutsOut.String())
		})
	}
}

func TestChooseFeature(t *testing.T) {
	for _, tt := range []struct {
		features WalletFeatures
		expected string
	}{
		{
			features: []string{"eth", "zksync"},
			expected: "eth",
		},
		{
			features: []string{"zksync", "eth"},
			expected: "eth",
		},
		{
			features: []string{"avalanche", "eth", "polygon"},
			expected: "eth",
		},
		{
			features: []string{"polygon", "zksync"},
			expected: "eth",
		},
		{
			features: []string{"zksync2", "zksync"},
			expected: "zksync-era",
		},
		{
			features: []string{"zksync-era", "eth"},
			expected: "zksync-era",
		},
		{
			features: []string{"[\"zksync\"]"},
			expected: "eth",
		},
		{
			features: []string{"[\"polygon\"]"},
			expected: "eth",
		},
		{
			features: []string{"[\"polygon,zksync\"]"},
			expected: "eth",
		},
	} {
		t.Run(fmt.Sprintf("%s-from-%s", tt.expected, strings.Join(tt.features, ",")), func(t *testing.T) {
			require.Equal(t, tt.expected, ChooseFeature(nil, NodeID{}, tt.features))
		})
	}

}
