// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/currency"
	"storj.io/storj/satellite/geoip"
	"storj.io/storj/shared/strictcsv"
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

// testOwed is what every invoice of the prepayment test says the node earned in
// the period, so anything above it in the payout is the prepayment.
const testOwed = "1000000"

// prepaymentComp is the compensation a prepayment test invoice records for the
// period, which is what the prepayment is a share of. An empty amount is zero.
type prepaymentComp struct {
	atRest    string
	get       string
	getRepair string
	putRepair string
	getAudit  string
}

// fill defaults every unset compensation amount to zero.
func (c prepaymentComp) fill() prepaymentComp {
	filled := c
	for _, amount := range []*string{
		&filled.atRest,
		&filled.get,
		&filled.getRepair,
		&filled.putRepair,
		&filled.getAudit,
	} {
		if *amount == "" {
			*amount = "0"
		}
	}
	return filled
}

// testPrepaymentInvoice is the single invoice a prepayment test case feeds to
// Prepare. Everything the prepayment does not depend on is left at zero: the
// prepayment is a share of the compensation alone, so the usage columns and the
// voluntary discount play no part in it. The surge percent does not enter the
// amount either, but a surge below 100 is refused outright (see prepayment), so
// it is settable.
type testPrepaymentInvoice struct {
	period string
	codes  string
	comp   prepaymentComp

	// surge defaults to "0", no surge, when empty.
	surge string

	// owed defaults to testOwed when empty.
	owed string

	// totalPaid and totalDistributed are what the previous periods left behind
	// for this one to distribute. Both default to zero when empty.
	totalPaid        string
	totalDistributed string
}

// fill applies the defaults of every unset field.
func (i testPrepaymentInvoice) fill() testPrepaymentInvoice {
	filled := i
	filled.comp = i.comp.fill()
	if filled.period == "" {
		filled.period = "2026-08"
	}
	if filled.owed == "" {
		filled.owed = testOwed
	}
	if filled.surge == "" {
		filled.surge = "0"
	}
	if filled.totalPaid == "" {
		filled.totalPaid = "0"
	}
	if filled.totalDistributed == "" {
		filled.totalDistributed = "0"
	}
	return filled
}

// row builds the invoice CSV row, in the same column order as testInvoiceRow.
func (i testPrepaymentInvoice) row() string {
	return strings.Join([]string{
		i.period,           // period
		testNodeID,         // node-id
		"2026-01-01",       // node-created-at
		"",                 // node-disqualified
		"",                 // node-gracefulexit
		testNodeWallet,     // node-wallet
		"",                 // node-wallet-features
		"",                 // node-last-ip
		i.codes,            // codes
		"0",                // usage-at-rest
		"0",                // usage-get
		"0",                // usage-put
		"0",                // usage-get-repair
		"0",                // usage-put-repair
		"0",                // usage-get-audit
		i.comp.atRest,      // comp-at-rest
		i.comp.get,         // comp-get
		"0",                // comp-put
		i.comp.getRepair,   // comp-get-repair
		i.comp.putRepair,   // comp-put-repair
		i.comp.getAudit,    // comp-get-audit
		i.surge,            // surge-percent
		i.owed,             // owed
		"0",                // held
		"0",                // disposed
		"0",                // total-held
		"0",                // total-disposed
		i.totalPaid,        // total-paid
		i.totalDistributed, // total-distributed
		"0",                // voluntary-discount
	}, ",") + "\n"
}

// paystubRow is the incomplete paystub written for this invoice, given what the
// period ends up paying and distributing.
func (i testPrepaymentInvoice) paystubRow(paid, possiblyDistributed string) string {
	return strings.Join([]string{
		i.period,            // period
		testNodeID,          // node-id
		i.codes,             // codes
		"0.000000",          // usage-at-rest
		"0",                 // usage-get
		"0",                 // usage-put
		"0",                 // usage-get-repair
		"0",                 // usage-put-repair
		"0",                 // usage-get-audit
		i.comp.atRest,       // comp-at-rest
		i.comp.get,          // comp-get
		"0",                 // comp-put
		i.comp.getRepair,    // comp-get-repair
		i.comp.putRepair,    // comp-put-repair
		i.comp.getAudit,     // comp-get-audit
		i.surge,             // surge-percent
		i.owed,              // owed
		"0",                 // held
		"0",                 // disposed
		paid,                // paid
		possiblyDistributed, // possibly-distributed
	}, ",") + "\n"
}

func codesOf(t *testing.T, s string) Codes {
	codes, err := CodesFromString(s)
	require.NoError(t, err)
	return codes
}

func TestPrepare_Prepayment(t *testing.T) {
	// 1.80 of at-rest compensation is 1.62 at the 90 percent the prepayment
	// prices data at rest at, which scaled by 30/36 is a 1.35 prepayment.
	const atRestComp = "1800000"

	// 1.20 of egress compensation is 0.60 at the half the prepayment pays for
	// egress, which scaled by 30/36 is a 0.50 prepayment.
	const egressComp = "1200000"

	// The prepayment is what the node earned for this period on top of the
	// invoice, so it is added to what the paystub records as paid as well as to
	// what the period distributes: it is not an advance, and the next period
	// does not recover it. Since these invoices carry no balance from earlier
	// periods (total-paid and total-distributed are zero), the paid amount and
	// the distributed amount coincide, and paid is what both columns hold. See
	// TestPrepare_PrepaymentLeavesNoNegativeBalance for the period after.
	for _, tt := range []struct {
		name       string
		prepayment bool
		period     string
		codes      string
		comp       prepaymentComp
		paid       string
	}{
		{
			// Without the flag the invoice is turned into a paystub as before,
			// however much the node earned.
			name: "disabled",
			comp: prepaymentComp{atRest: atRestComp, getRepair: egressComp},
			paid: testOwed,
		},
		{
			name:       "at-rest compensation only",
			prepayment: true,
			comp:       prepaymentComp{atRest: atRestComp},
			paid:       "2350000",
		},
		{
			// Egress, repair egress and audit egress are all prepaid at half of
			// what the period compensated them with.
			name:       "egress compensation only",
			prepayment: true,
			comp:       prepaymentComp{get: egressComp, getRepair: egressComp, getAudit: egressComp},
			paid:       "2500000",
		},
		{
			name:       "all compensation",
			prepayment: true,
			comp:       prepaymentComp{atRest: atRestComp, get: egressComp, getRepair: egressComp, getAudit: egressComp},
			paid:       "3850000",
		},
		{
			// Repair ingress is not prepaid.
			name:       "repair ingress compensation only",
			prepayment: true,
			comp:       prepaymentComp{putRepair: egressComp},
			paid:       testOwed,
		},
		{
			// A node that earned nothing for the period earns no prepayment.
			name:       "no compensation",
			prepayment: true,
			paid:       testOwed,
		},
		{
			// The prepayment is a share of what the invoice compensated the
			// node with, so a node that lowered its own prices with self-signed
			// price tags (see tag_rates.go) is prepaid at the reduced price too:
			// two thirds of the at-rest compensation of the case above leave two
			// thirds of its prepayment.
			name:       "voluntarily reduced at-rest price",
			prepayment: true,
			comp:       prepaymentComp{atRest: "1200000"},
			paid:       "1900000",
		},
		{
			// A node that waived payment for a dimension earns no prepayment
			// for it either.
			name:       "waived at-rest price",
			prepayment: true,
			comp:       prepaymentComp{atRest: "0"},
			paid:       testOwed,
		},
		{
			// 3 micro-units of at-rest compensation prepay 2.25 micro-units,
			// which is truncated to the micro-unit everything is paid in.
			name:       "prepayment below a micro-unit is truncated",
			prepayment: true,
			comp:       prepaymentComp{atRest: "3"},
			paid:       "1000002",
		},
		{
			// The period only decides what the node earned, so the same
			// compensation is prepaid the same in a 30 day period as in the 31
			// day one the other cases use.
			name:       "shorter period pays the same",
			prepayment: true,
			period:     "2026-06",
			comp:       prepaymentComp{atRest: atRestComp},
			paid:       "2350000",
		},
		{
			// The prepayment buys continued participation, so the nodes that are
			// on their way out of the network do not get one.
			name:       "disqualified node",
			prepayment: true,
			codes:      string(Disqualified),
			comp:       prepaymentComp{atRest: atRestComp, getRepair: egressComp},
			paid:       testOwed,
		},
		{
			name:       "gracefully exited node",
			prepayment: true,
			codes:      string(GracefulExit),
			comp:       prepaymentComp{atRest: atRestComp},
			paid:       testOwed,
		},
		{
			name:       "gracefully exiting node",
			prepayment: true,
			codes:      string(GracefulExiting),
			comp:       prepaymentComp{atRest: atRestComp},
			paid:       testOwed,
		},
		{
			// A node that left before the period is already covered by
			// GracefulExit or GracefulExiting in practice, since
			// exit_finished_at is only ever written for a node that has
			// exit_initiated_at set. Exited is listed anyway so the guarantee
			// does not depend on an invariant maintained in another package:
			// the Exited branch zeroes owed but leaves CompAtRest gross, so a
			// node reaching here would be prepaid for a period it earned
			// nothing in.
			name:       "node that left the network before the period",
			prepayment: true,
			codes:      string(Exited),
			comp:       prepaymentComp{atRest: atRestComp},
			paid:       testOwed,
		},
		{
			// The prepayment is a share of the gross compensation, which for a
			// node in withholding is several times what the period owes it, so
			// prepaying it would hand over the escrow the withholding exists to
			// hold back while the paystub still records it as held.
			name:       "node in withholding",
			prepayment: true,
			codes:      string(InWithholding),
			comp:       prepaymentComp{atRest: atRestComp},
			paid:       testOwed,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			invoice := testPrepaymentInvoice{
				period: tt.period,
				codes:  tt.codes,
				comp:   tt.comp,
			}.fill()

			invoicesIn := strings.NewReader(testInvoicesHeader + "\n" + invoice.row())
			paystubsOut := new(bytes.Buffer)
			payoutsOut := new(bytes.Buffer)

			err := Prepare(invoicesIn, paystubsOut, payoutsOut, PrepareConfig{
				SkipOFAC:   true,
				Prepayment: tt.prepayment,
			})
			require.NoError(t, err)

			require.Equal(t,
				testPaystubsHeader+"\n"+invoice.paystubRow(tt.paid, tt.paid),
				paystubsOut.String())

			// the prepayment goes out with the payout of the period
			paid, err := strconv.ParseInt(tt.paid, 10, 64)
			require.NoError(t, err)
			// a node that is leaving already had a mandatory payout, prepayment
			// or not
			mandatory := strconv.FormatBool(isMandatory(codesOf(t, tt.codes)))
			require.Equal(t,
				testPrePayoutsHeader+"\n"+testNodeWallet+","+
					currency.NewMicroUnit(paid).FloatString()+",eth,"+mandatory+",false\n",
				payoutsOut.String())
		})
	}
}

// The prepayment is a share of the pre-surge compensation while the invoice is
// owed a post-surge amount, so a surge below 100 would pay the node more than
// the period says it earned: at a surge of 50 an at-rest-dominated node is owed
// half its compensation and would still be prepaid 0.75 of it. The combination
// is refused rather than silently reinterpreted, since scaling the prepayment
// by the surge would break the fixed 30/36 at 1.35 model for a surge over 100.
func TestPrepare_PrepaymentSurge(t *testing.T) {
	// 1.80 of at-rest compensation prepays 1.35, the same constant
	// TestPrepare_Prepayment uses.
	const atRestComp = "1800000"

	for _, tt := range []struct {
		name  string
		surge string
		// paid is what the paystub records; empty means Prepare must fail.
		paid string
	}{
		{
			name:  "no surge",
			surge: "0",
			paid:  "2350000",
		},
		{
			// A surge of exactly 100 leaves owed unscaled, so it is the same
			// case as no surge at all.
			name:  "surge of 100",
			surge: "100",
			paid:  "2350000",
		},
		{
			// Above 100 the prepayment deliberately does not inflate with the
			// surge; the invoice already carries the surged owed.
			name:  "surge above 100",
			surge: "150",
			paid:  "2350000",
		},
		{
			name:  "surge below 100",
			surge: "50",
		},
		{
			name:  "surge of 1",
			surge: "1",
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			invoice := testPrepaymentInvoice{
				surge: tt.surge,
				comp:  prepaymentComp{atRest: atRestComp},
			}.fill()

			paystubsOut := new(bytes.Buffer)
			payoutsOut := new(bytes.Buffer)
			err := Prepare(
				strings.NewReader(testInvoicesHeader+"\n"+invoice.row()),
				paystubsOut, payoutsOut,
				PrepareConfig{SkipOFAC: true, Prepayment: true})

			if tt.paid == "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), "surge")
				return
			}
			require.NoError(t, err)
			require.Equal(t,
				testPaystubsHeader+"\n"+invoice.paystubRow(tt.paid, tt.paid),
				paystubsOut.String())
		})
	}
}

// Without the prepayment a surge below 100 is none of Prepare's business: the
// invoice is turned into a paystub as before, so the guard cannot break a
// surged run that does not ask for a prepayment.
func TestPrepare_SurgeWithoutPrepayment(t *testing.T) {
	invoice := testPrepaymentInvoice{
		surge: "50",
		comp:  prepaymentComp{atRest: "1800000"},
	}.fill()

	paystubsOut := new(bytes.Buffer)
	payoutsOut := new(bytes.Buffer)
	require.NoError(t, Prepare(
		strings.NewReader(testInvoicesHeader+"\n"+invoice.row()),
		paystubsOut, payoutsOut,
		PrepareConfig{SkipOFAC: true}))

	require.Equal(t,
		testPaystubsHeader+"\n"+invoice.paystubRow(testOwed, testOwed),
		paystubsOut.String())
}

// A prepayment must not leave a negative balance behind. The period after a
// prepaid one distributes Owed + (TotalPaid - TotalDistributed); if the
// prepayment were only distributed and not also paid, that term would be
// negative by exactly the prepayment, and a node that earns less in the next
// period than it was prepaid in this one - routine churn - would be handed a
// negative payout in the prepayouts CSV and a negative distributed amount in
// its paystub.
func TestPrepare_PrepaymentLeavesNoNegativeBalance(t *testing.T) {
	prepare := func(t *testing.T, invoice testPrepaymentInvoice) IncompletePaystub {
		paystubsOut := new(bytes.Buffer)
		payoutsOut := new(bytes.Buffer)

		require.NoError(t, Prepare(
			strings.NewReader(testInvoicesHeader+"\n"+invoice.fill().row()),
			paystubsOut, payoutsOut,
			PrepareConfig{SkipOFAC: true, Prepayment: true},
		))

		paystubs, err := ReadIncompletePaystubs(paystubsOut)
		require.NoError(t, err)
		require.Len(t, paystubs, 1)

		var payouts []Prepayout
		require.NoError(t, strictcsv.Read(payoutsOut, &payouts))
		require.Len(t, payouts, 1)
		require.Equal(t, paystubs[0].PossiblyDistributed, currency.MicroUnit(payouts[0].Amount))

		return paystubs[0]
	}

	// 1.80 of at-rest compensation prepays 1.35 on top of the 1.00 owed.
	first := prepare(t, testPrepaymentInvoice{
		period: "2026-08",
		comp:   prepaymentComp{atRest: "1800000"},
	})
	require.Equal(t, int64(2350000), first.Paid.Value())
	require.Equal(t, int64(2350000), first.PossiblyDistributed.Value())

	// The next period for the same node, which earned nothing at all: the
	// totals of the period above are all the node has behind it.
	second := prepare(t, testPrepaymentInvoice{
		period:           "2026-09",
		owed:             "0",
		totalPaid:        strconv.FormatInt(first.Paid.Value(), 10),
		totalDistributed: strconv.FormatInt(first.PossiblyDistributed.Value(), 10),
	})
	require.Zero(t, second.Paid.Value())
	require.Zero(t, second.PossiblyDistributed.Value(),
		"the prepayment of the previous period must not be clawed back out of this one")
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
