#!/bin/bash
set -ueo pipefail

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)
go install storj.io/storj/cmd/uplink
go install storj.io/storj/cmd/identity

identity create . --identity-dir $TMPDIR --difficulty 12

uplink setup --api-key="abc123" --identity-dir $TMPDIR \
--satellite-addr "localhost:10000" \
--config-dir $TMPDIR

cat $TMPDIR/config.yaml
uplink mb sj://testbucket/ --config-dir $TMPDIR
# uplink ls sj://testbucket/ --config-dir $TMPDIR