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

const (
	testNodeID     = "1SkB92YpWm4Q2ijQHH34cqbKkCZWszsiQgHVjtNeFExggbYvy"
	testNodeWallet = "0x0123456789abcdef0123456789abcdef01234567"
)

// testInvoiceRow builds an invoice CSV row matching testInvoicesHeader. The
// node is owed 1 unit and has never been paid, so it is only the OFAC gate
// that decides whether a payout is written.
func testInvoiceRow(nodeLastIP string) string {
	return strings.Join([]string{
		"2026-08",      // period
		testNodeID,     // node-id
		"2026-01-01",   // node-created-at
		"",             // node-disqualified
		"",             // node-gracefulexit
		testNodeWallet, // node-wallet
		"",             // node-wallet-features
		nodeLastIP,     // node-last-ip
		"",             // codes
		"0",            // usage-at-rest
		"0",            // usage-get
		"0",            // usage-put
		"0",            // usage-get-repair
		"0",            // usage-put-repair
		"0",            // usage-get-audit
		"1000000",      // comp-at-rest
		"0",            // comp-get
		"0",            // comp-put
		"0",            // comp-get-repair
		"0",            // comp-put-repair
		"0",            // comp-get-audit
		"0",            // surge-percent
		"1000000",      // owed
		"0",            // held
		"0",            // disposed
		"0",            // total-held
		"0",            // total-disposed
		"0",            // total-paid
		"0",            // total-distributed
		"0",            // voluntary-discount
	}, ",") + "\n"
}

// testPaystubRow is the incomplete paystub written for testInvoiceRow when the
// payout is not zeroed by a sanction.
func testPaystubRow() string {
	return strings.Join([]string{
		"2026-08",  // period
		testNodeID, // node-id
		"",         // codes
		"0.000000", // usage-at-rest
		"0",        // usage-get
		"0",        // usage-put
		"0",        // usage-get-repair
		"0",        // usage-put-repair
		"0",        // usage-get-audit
		"1000000",  // comp-at-rest
		"0",        // comp-get
		"0",        // comp-put
		"0",        // comp-get-repair
		"0",        // comp-put-repair
		"0",        // comp-get-audit
		"0",        // surge-percent
		"1000000",  // owed
		"0",        // held
		"0",        // disposed
		"1000000",  // paid
		"1000000",  // possibly-distributed
	}, ",") + "\n"
}

func TestPrepare(t *testing.T) {
	for _, tt := range []struct {
		name            string
		headerOverride  string
		invoicesIn      string
		paystubsOut     string
		payoutsOut      string
		geoIPDBs        []*geoip.MaxmindDB
		skipOFAC        bool
		allowUnscreened bool
		err             string
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
				`"usage-put-repair"` +
				`] missing from CSV`,
		},
		{
			// No GeoIP database can screen a node without an IP, so the run
			// must fail closed rather than pay out an unscreened wallet.
			name:       "missing node-last-ip refuses payouts",
			invoicesIn: testInvoiceRow(""),
			err:        "refusing to write payouts: 1 nodes could not be OFAC-screened (use AllowUnscreened to override)",
		},
		{
			// A garbage IP is indistinguishable from a missing one: both leave
			// the node unscreened, so the gate must still hold.
			name:       "invalid node-last-ip refuses payouts",
			invoicesIn: testInvoiceRow("not-an-ip"),
			err:        "refusing to write payouts: 1 nodes could not be OFAC-screened (use AllowUnscreened to override)",
		},
		{
			// Same input, but the operator explicitly accepted the risk.
			name:            "missing node-last-ip with AllowUnscreened",
			invoicesIn:      testInvoiceRow(""),
			allowUnscreened: true,
			paystubsOut:     testPaystubRow(),
			payoutsOut:      testNodeWallet + ",1.000000,eth,false,false\n",
		},
		{
			// With screening off entirely the gate is not consulted at all.
			name:        "missing node-last-ip with SkipOFAC",
			invoicesIn:  testInvoiceRow(""),
			skipOFAC:    true,
			paystubsOut: testPaystubRow(),
			payoutsOut:  testNodeWallet + ",1.000000,eth,false,false\n",
		},
		{
			// A valid IP with no GeoIP database loaded still counts as
			// unscreened; the gate must not fall open just because the IP
			// parsed.
			name:       "valid node-last-ip without geoip database",
			invoicesIn: testInvoiceRow("1.2.3.4"),
			err:        "refusing to write payouts: 1 nodes could not be OFAC-screened (use AllowUnscreened to override)",
		},
		{
			// Invoices generated before node-address was dropped are rejected
			// outright: prepare deliberately stays strict about schema skew on
			// the money path, so such a period must be regenerated.
			name:           "legacy node-address column",
			headerOverride: testInvoicesHeader + ",node-address",
			err:            `strictcsv: CSV header "node-address" is not mapped to struct field`,
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
				GeoIPDBs:        tt.geoIPDBs,
				SkipOFAC:        tt.skipOFAC,
				AllowUnscreened: tt.allowUnscreened,
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
