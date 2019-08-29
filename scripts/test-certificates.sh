#!/usr/bin/env bash
set -ueo pipefail
source $(dirname $0)/utils.sh

TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)

  # TODO: find a better way
kill_certificates_server() {
  killall "certificates"
}

cleanup_int() {
  kill_certificates_server
  exit
}

cleanup_exit() {
  rm -rf "$TMPDIR"
  echo "cleaned up test successfully"
}

trap cleanup_exit EXIT
trap cleanup_int INT

_certificates() {
  certificates --config-dir "$CERTS_DIR" "$@"
}

_identity() {
  subcommand=$1
  shift
  identity --log.level warn $subcommand --identity-dir "$IDENTS_DIR" "$@"
}

export_auths() {
  _certificates auth export
}

IDENTS_DIR=$TMPDIR/identities
CERTS_DIR=$TMPDIR/certificates

_certificates setup --log.level warn

for i in {0..4}; do
  email="testuser${i}@mail.example"
  ident_name="testidentity${i}"

  _identity create $ident_name --difficulty 0 --concurrency 1 >/dev/null

  if [[ i -gt 0 ]]; then
    _certificates auth create "$i" "$email"
  fi
done

#
exported_auths=$(export_auths)
_certificates run --signer.min-difficulty 0 &

sleep 1

for i in {1..4}; do
  email="testuser${i}@mail.example"
  ident_name="testidentity${i}"

  token=$(echo "$exported_auths" | grep "$email" | head -n 1 | awk -F , '{print $2}')
  _identity authorize --signer.address 127.0.0.1:7777 "$ident_name" "$token" > /dev/null
done

# NB: Certificates server uses bolt by default so it must be shut down before we can export.
kill_certificates_server

# Expect 10 authorizations total.
auths=$(echo $(export_auths))
require_lines 10 "$auths" $LINENO

for i in {1..4}; do
  email="testuser${i}@mail.example"
  claimed_auth_count=0

  # Expect number of auths for a given user to equal the identity/email number.
  # (e.g. testidentity3/testuser3@mail.example should have 3 auths)
  match_auths=$(echo $auths | grep "$email" )
  require_lines $i "$match_auths" $LINENO

  for auth in $match_auths; do
    claimed=$(echo $auth | awk -F , '{print $3}')
    if [[ $claimed == "true" ]]; then
      ((++claimed_auth_count))
      continue
    fi
    # Expect unclaimed auths to have "false" as the third field.
    require_equal "false" "$claimed" $LINENO
  done

  # Expect 4 auths (one for each user) to be claimed.
  require_equal "4" "$claimed_auth_count" $LINENO
done

echo "TEST COMPLETED SUCCESSFULLY!"
