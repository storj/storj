#!/usr/bin/env bash
set -ueo pipefail
set +x

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# setup tmpdir for testfiles and cleanup
TMP=$(mktemp -d -t tmp.XXXXXXXXXX)
cleanup(){
	rm -rf "$TMP"
}
trap cleanup EXIT

export PATH=$TMP/bin:$PATH
echo "Running test-sim"
make -C "$SCRIPTDIR"/.. install-sim

export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}
STORJ_SIM_REDIS=${STORJ_SIM_REDIS:-""}
# STORJ_CONSOLE_payments_stripe-coin-payments_coinpayments-private-key="5366b14A7Dc5A1b0FCc3C8845c5d903E8c6b6360de5f3667AD8B58f5E8cC017c"
STORJ_CONSOLE_STATIC_DIR="$SCRIPTDIR/../web/satellite"
# setup the network
# if postgres connection string is set as STORJ_SIM_POSTGRES then use that for testing
if [ -z ${STORJ_SIM_POSTGRES} ]; then
	storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network setup
else
	storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup
fi

# run UI tests
echo "section tests start"
apt-get -y install chromium
export DEBIAN_FRONTEND="noninteractive"
apt-get -y install xorg xvfb gtk2-engines-pixbuf dbus-x11 xfonts-base xfonts-100dpi xfonts-75dpi xfonts-cyrillic xfonts-scalable imagemagick x11-apps

echo "wormhole installing ..............................................................................................."
apt-get -y install wormhole
Xvfb -ac :99 -screen 0 1280x1024x16 & export DISPLAY=:99

pushd "$SCRIPTDIR/../web/satellite/"
echo "npm install starts..........................................................................................."
npm install
echo "npm run build starts...................................................................................................."
npm run build
popd

storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network run &
go test "$SCRIPTDIR"/tests/UITests/.
wormhole send "$SCRIPTDIR"/tests/UITests/screenshots/my1.png
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network destroy

# setup the network with ipv6
#storj-sim -x --host "::1" network setup
# aws-cli doesn't support gateway with ipv6 address, so change it to use localhost
#find "$STORJ_NETWORK_DIR"/gateway -type f -name config.yaml -exec sed -i 's/server.address: "\[::1\]/server.address: "127.0.0.1/' '{}' +
# run aws-cli tests using ipv6
#storj-sim -x --host "::1" network test bash "$SCRIPTDIR"/test-sim-aws.sh
#storj-sim -x network destroy
