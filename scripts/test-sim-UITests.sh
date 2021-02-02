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
# mirroring install-sim from the Makefile since it won't work on private Jenkins
install_sim(){
    local bin_dir="${TMP}/bin"
    mkdir -p ${bin_dir}

    go build -race -v -o ${bin_dir}/storagenode ./cmd/storagenode >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/satellite ./cmd/satellite >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/storj-sim ./cmd/storj-sim >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/versioncontrol ./cmd/versioncontrol >/dev/null 2>&1

    go build -race -v -o ${bin_dir}/uplink ./cmd/uplink >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/identity ./cmd/identity >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/certificates ./cmd/certificates >/dev/null 2>&1

    rm -rf .build/gateway-tmp
    mkdir -p .build/gateway-tmp
    pushd .build/gateway-tmp
        go mod init gatewaybuild && GOBIN=${bin_dir} GO111MODULE=on go get storj.io/gateway@latest
    popd
}

pushd $SCRIPTDIR
    echo "Running test-sim"
    
    if [ -d "$SCRIPTDIR/storj" ]; then 
      rm -Rf $SCRIPTDIR/storj; 
    fi
    
    git clone https://github.com/storj/storj.git --depth 1

    pushd ./storj
        install_sim
    popd
popd

export PATH=$TMP/bin:$PATH
echo "Running test-sim"
make -C "$SCRIPTDIR"/.. install-sim

export STORJ_NETWORK_DIR=$TMP

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}
STORJ_SIM_REDIS=${STORJ_SIM_REDIS:-""}
# STORJ_CONSOLE_payments_stripe-coin-payments_coinpayments-private-key="5366b14A7Dc5A1b0FCc3C8845c5d903E8c6b6360de5f3667AD8B58f5E8cC017c"
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
apt-get -y install xorg xvfb gtk2-engines-pixbuf
apt-get -y install dbus-x11 xfonts-base xfonts-100dpi xfonts-75dpi xfonts-cyrillic xfonts-scalable
apt-get -y install imagemagick x11-apps
apt-get -y install nodejs
npm install --prefix "$SCRIPTDIR"/../web/satellite
npm run build --prefix "$SCRIPTDIR"/../web/satellite
Xvfb -ac :99 -screen 0 1280x1024x16 & export DISPLAY=:99
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network run &
go test "$SCRIPTDIR"/tests/UITests/.
storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network destroy

# setup the network with ipv6
#storj-sim -x --host "::1" network setup
# aws-cli doesn't support gateway with ipv6 address, so change it to use localhost
#find "$STORJ_NETWORK_DIR"/gateway -type f -name config.yaml -exec sed -i 's/server.address: "\[::1\]/server.address: "127.0.0.1/' '{}' +
# run aws-cli tests using ipv6
#storj-sim -x --host "::1" network test bash "$SCRIPTDIR"/test-sim-aws.sh
#storj-sim -x network destroy
