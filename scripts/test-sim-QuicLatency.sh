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
export STORJ_NETWORK_DIR=$TMP

# mirroring install-sim from the Makefile since it won't work on private Jenkins
install_sim(){
    local bin_dir="${TMP}/bin"
    mkdir -p ${bin_dir}

    go build -race -v -o ${bin_dir}/storagenode ./cmd/storagenode
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


if [ "$HOSTNAME" = $STORJ_NETWORK_HOST4 ]; then
    # Go version and install older versions
    go version
    which go
    #go1.14 version
    #which go1.14
    #go install golang.org/dl/go1.15@latest && go1.15 download
    #go1.15 version

    pushd $SCRIPTDIR
        echo "Running test-sim"
        echo "Running $SCRIPTDIR"

        if [ -d "$SCRIPTDIR/storj" ]; then
          rm -Rf $SCRIPTDIR/storj;
        fi

        git clone https://github.com/storj/storj.git --depth 1

        pushd ./storj
            git status
            install_sim
        popd
    popd

    # Copy uplink for other container
    cp $TMP/bin/uplink ./data/

    STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
    STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}
    STORJ_SIM_REDIS=${STORJ_SIM_REDIS:-""}
    # setup the network
    # if postgres connection string is set as STORJ_SIM_POSTGRES then use that for testing
    if [ -z ${STORJ_SIM_POSTGRES} ]; then
        storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network setup #--storage-nodes 1 --identities 1
    else
        storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network --postgres=$STORJ_SIM_POSTGRES setup #--storage-nodes 1 --identities 1
    fi

    echo "metainfo.rate-limiter.enabled: false" >> $TMP/satellite/0/config.yaml
    #echo "metainfo.rs: 1/1/1/1-256 B" >> $TMP/satellite/0/config.yaml  # disable RS -> one piece

    cat $TMP/satellite/0/config.yaml

    #storj-sim -x --satellites 1 --storage-nodes 1 --host $STORJ_NETWORK_HOST4 network run &
    storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network run &

    sleep 30 # wait for satellite setup

    echo get access grant
    storj-sim network env
    #storj-sim network env GATEWAY_0_ACCESS #--storage-nodes 1
    storj-sim network env GATEWAY_0_ACCESS | tee  ./data/access.txt

    #uplink --config-dir=$TMP/uplink import $(storj-sim network env GATEWAY_0_ACCESS --storage-nodes 1)
    uplink --config-dir=$TMP/uplink import $(storj-sim network env GATEWAY_0_ACCESS)

    uplink --config-dir=$TMP/uplink access inspect | tee ./data/inspect.txt

    echo "Set uplink config"
    echo "advanced: true" >> $TMP/uplink/config.yaml
    echo "log.caller: true" >> $TMP/uplink/config.yaml
    echo "log.development: true" >> $TMP/uplink/config.yaml
    echo "log.level: debug" >> $TMP/uplink/config.yaml
    echo "log.stack: true" >> $TMP/uplink/config.yaml

    echo copy uplink config
    cp $TMP/uplink/config.yaml ./data/

    sleep infinity

    storj-sim -x --satellites 1 --host $STORJ_NETWORK_HOST4 network destroy
fi

if [ "$HOSTNAME" = uplink ]; then   
    #install tcpdump
    apt-get -y install tcpdump

    head -c 64M </dev/urandom > /tmp/64M
    
    # wait for satellite setup
    COUNTER=0
    while [ ! -f ./data/config.yaml ]; do
      if (( COUNTER > 300)); then
        break
      fi
      let COUNTER=COUNTER+5
      sleep 5;
    done

    echo "### Docker Logs of Satellite container #################################################################"
    docker logs $STORJ_NETWORK_HOST4
    
    echo create bucket
    ./data/uplink --config-dir=./data/ mb sj://test
    ./data/uplink --config-dir=./data/ ls
   
    for i in 0 1 2 3  
    do
      echo start with an extra of $((25*i*2))ms
      tc qdisc replace dev eth0 root netem delay $((25*i))ms
      docker exec $STORJ_NETWORK_HOST4 tc qdisc replace dev eth0 root netem delay $((25*i))ms
      
      ping -c1 $STORJ_NETWORK_HOST4
      
      ## TCP Run
      tcpdump -i any -s 65535 -w ./data/tcp_${BUILD_NUMBER}_$((25*i*2)).cap port not 22 &
      sleep 2.5
      echo "### Upload TCP $((25*i*2))ms ######################################################################"
      ./data/uplink --config-dir=./data/ --debug.trace-out ./data/out_tcp_up_${BUILD_NUMBER}_$((25*i*2)).svg cp /tmp/64M sj://test
      # optinal parameter: --profile.cpu cpu.profile --progress=false
      sleep 2.5
      echo "### Download TCP $((25*i*2))ms ######################################################################"
      ./data/uplink --config-dir=./data/ --debug.trace-out ./data/out_tcp_dl_${BUILD_NUMBER}_$((25*i*2)).svg cp sj://test/64M /tmp/64M.dl

      ##interrupt it:
      sleep 2.5
      kill -2 $(ps -e | pgrep tcpdump)

      ./data/uplink --config-dir=./data/ rm sj://test/64M
      rm /tmp/64M.dl
      sleep 2.5

      ## QUIC Run
      tcpdump -i any -s 65535 -w ./data/quic_${BUILD_NUMBER}_$((25*i*2)).cap port not 22 &
      sleep 2.5
      echo "### Upload UDP $((25*i*2))ms ######################################################################"
      ./data/uplink --config-dir=./data/ cp --debug.trace-out ./data/out_quic_ul_${BUILD_NUMBER}_$((25*i*2)).svg --client.enable-quic /tmp/64M sj://test
      sleep 2.5
      echo "### Download UDP $((25*i*2))ms ######################################################################"
      ./data/uplink --config-dir=./data/ cp --debug.trace-out ./data/out_quic_dl_${BUILD_NUMBER}_$((25*i*2)).svg --client.enable-quic sj://test/64M /tmp/64M.dl

      #interrupt it:
      sleep 2.5
      kill -2 $(ps -e | pgrep tcpdump)
    done

    sleep 1     
        
fi
