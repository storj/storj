// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/storj/satellite/compensation"
)

// withExtraColumn adds a column not mapped to any Invoice field, simulating an
// invoices CSV produced by a satellite version whose schema still had it.
func withExtraColumn(t *testing.T, csv, header, value string) string {
	t.Helper()
	lines := strings.Split(strings.TrimRight(csv, "\n"), "\n")
	require.Len(t, lines, 2, "expected a header and a single record")
	return lines[0] + "," + header + "\n" + lines[1] + "," + value + "\n"
}

func invoicesCSV(t *testing.T) string {
	t.Helper()
	period, err := compensation.PeriodFromString("2026-08")
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, compensation.WriteInvoices(&buf, []compensation.Invoice{{
		Period:     period,
		NodeID:     compensation.NodeID(testrand.NodeID()),
		NodeWallet: "0x0123456789012345678901234567890123456789",
	}}))
	return buf.String()
}

func TestReadInvoicesRejectsUnmappedColumn(t *testing.T) {
	csv := withExtraColumn(t, invoicesCSV(t), "node-address", "127.0.0.1:7777")

	// ReadInvoices feeds the money-producing paths (prepare, finalize), so an
	// unrecognized column has to fail the run rather than be discarded.
	_, err := compensation.ReadInvoices(strings.NewReader(csv))
	require.EqualError(t, err, `strictcsv: CSV header "node-address" is not mapped to struct field`)
}

func TestReadInvoicesLenientIgnoresUnmappedColumn(t *testing.T) {
	original := invoicesCSV(t)

	strict, err := compensation.ReadInvoices(strings.NewReader(original))
	require.NoError(t, err)

	lenient, err := compensation.ReadInvoicesLenient(strings.NewReader(
		withExtraColumn(t, original, "node-address", "127.0.0.1:7777")))
	require.NoError(t, err)

	// The retired column is dropped; every mapped field is unaffected by it.
	require.Equal(t, strict, lenient)
}
