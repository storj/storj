#!/usr/bin/env bash

set -ueo pipefail
set +x

cleanup(){
    for version in ${unique_versions}; do
        git worktree remove --force $(version_dir $version)
    done
    rm -rf "$TMP"
    echo "cleaned up test successfully"
}
trap cleanup EXIT

#Stage1:
    #satellite-version: v0.16.2
    #uplink-version: v0.16.2
    #storagenode-version:
        #5 nodes on version v0.16.2
        #5 nodes on version v0.15.4
#Stage2:
    #satellite-version: master
    #uplink-version: master, v0.20.2, v0.19.7, v0.18.0, v0.17.1, v0.16.2, v0.15.4
    #storagenode-version:
        #5 nodes on version master
        #5 nodes on version v0.15.4


stage1_sat_version="v0.16.2"
stage1_uplink_version="v0.16.2"
stage1_storagenode_versions="v0.16.2 v0.16.2 v0.16.2 v0.16.2 v0.16.2 v0.15.4 v0.15.4 v0.15.4 v0.15.4 v0.15.4"
stage2_sat_version="v0.20.2"
stage2_uplink_version="v0.16.2"
stage2_storagenode_versions="v0.20.2 v0.20.2 v0.20.2 v0.20.2 v0.20.2 v0.19.7 v0.19.7 v0.19.7 v0.19.7 v0.19.7"

TMP=$(mktemp -d -t tmp.XXXXXXXXXX)

find_unique_versions(){
    echo "$*" | tr " " "\n" | sort | uniq
}

version_dir(){
    echo "${TMP}/${1}"
}

# Set up each environment
unique_versions=$(find_unique_versions $stage1_sat_version $stage1_uplink_version $stage1_storagenode_versions $stage2_sat_version $stage2_uplink_version $stage2_storagenode_versions)

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}

if [ -z ${STORJ_SIM_POSTGRES} ]; then
    echo "STORJ_SIM_POSTGRES is required for the satellite DB. Example: STORJ_SIM_POSTGRES=postgres://[user]:[pass]@[host]/[db]?sslmode=disable"
    exit 1
fi

echo "Setting up environments for versions" ${unique_versions}
for version in ${unique_versions}; do
    dir=$(version_dir ${version})
    bin_dir=${dir}/bin

    git worktree add -f ${dir} ${version}
    rm ${dir}/internal/version/release.go
    GOBIN=${bin_dir} make -C "${dir}" install-sim
    PATH=${bin_dir}:$PATH storj-sim -x --host="${STORJ_NETWORK_HOST4}" --postgres="$STORJ_SIM_POSTGRES" --config-dir "${dir}/local-network" network setup

    echo "Finished setting up:" $(ls ${dir}/local-network)
done

# stage 1
# select storj-sim directory for stage 1 satellite version
stage1_dir=$(version_dir ${stage1_sat_version})

# iterate over every storagenode for that instance of storj-sim and symlink to storagenode binary for desired stage 2 storagenode version
counter=0
for sn_version in ${stage1_storagenode_versions}; do
    $sn_version_dir=$(version_dir ${sn_version})

    PATH=$sn_version_dir/bin:$PATH desired_sn_cfg_dir=`storj-sim network env STORAGENODE_${counter}_DIR`
    PATH=$stage1_dir/bin:$PATH stage1_sn_cfg_dir=`storj-sim network env STORAGENODE_${counter}_DIR`
    
    # link binary and copy config.yaml for desired version
    ln $sn_version_dir/bin/storagenode $stage1_sn_cfg_dir/storagenode
    mv $stage1_sn_cfg_dir/config.yaml $stage1_sn_cfg_dir/orignial-config.yaml
    cp $desired_sn_cfg_dir/config.yaml $stage1_sn_cfg_dir
    # TODO remove symlink and copy back original-config.yaml after stage 1 finishes

    let counter+=1
done

# use desired uplink binary and config
$ul_version_dir=$(version_dir ${stage1_uplink_version})
PATH=$ul_version_dir/bin:$PATH desired_ul_cfg_dir=`storj-sim network env GATEWAY_0_DIR`
PATH=$stage1_dir/bin:$PATH stage1_ul_cfg_dir=`storj-sim network env GATEWAY_0_DIR`
mv $stage1_ul_cfg_dir/config.yaml $stage1_ul_cfg_dir/orignial-config.yaml
cp $desired_ul_cfg_dir/config.yaml $stage1_ul_cfg_dir
# TODO copy back original-config.yaml after stage 1 finshes
mv $stage1_dir/bin/uplink $stage1_dir/bin/original-uplink
ln $ul_version_dir/bin/uplink $stage1_dir/bin/uplink
# TODO copy back original uplink binary and remove symlink

# run backwards compatibility test with stage 1 uplink version
# run upload part of backward compatibility tests (TODO set SCRIPTDIR)
PATH=$stage1_dir/bin:$PATH storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh upload

# stage 2
# select storj-sim directory for stage 2 satellite version
stage2_dir=$(version_dir ${stage2_sat_version})

# iterate over every storagenode for that instance of storj-sim and symlink to storagenode binary for desired stage 2 storagenode version
counter=0
for sn_version in ${stage2_storagenode_versions}; do
    $sn_version_dir=$(version_dir ${sn_version})

    PATH=$sn_version_dir/bin:$PATH desired_sn_cfg_dir=`storj-sim network env STORAGENODE_${counter}_DIR`
    PATH=$stage2_dir/bin:$PATH stage2_sn_cfg_dir=`storj-sim network env STORAGENODE_${counter}_DIR`
    
    # link binary and copy config.yaml for desired version
    ln $sn_version_dir/bin/storagenode $stage2_sn_cfg_dir/storagenode
    mv $stage2_sn_cfg_dir/config.yaml $stage2_sn_cfg_dir/orignial-config.yaml
    cp $desired_sn_cfg_dir/config.yaml $stage2_sn_cfg_dir
    # TODO remove symlink and copy back original-config.yaml after stage 2 finishes

    let counter+=1
done

# use desired uplink binary and config
$ul_version_dir=$(version_dir ${stage2_uplink_version})
PATH=$ul_version_dir/bin:$PATH desired_ul_cfg_dir=`storj-sim network env GATEWAY_0_DIR`
PATH=$stage2_dir/bin:$PATH stage2_ul_cfg_dir=`storj-sim network env GATEWAY_0_DIR`
mv $stage2_ul_cfg_dir/config.yaml $stage2_ul_cfg_dir/orignial-config.yaml
cp $desired_ul_cfg_dir/config.yaml $stage2_ul_cfg_dir
# TODO copy back original-config.yaml after stage 2 finshes
mv $stage2_dir/bin/uplink $stage2_dir/bin/original-uplink
ln $ul_version_dir/bin/uplink $stage2_dir/bin/uplink
# TODO copy back original uplink binary and remove symlink

# run backwards compatibility test with stage 2 uplink version
# run download part of backward compatibility tests (TODO set SCRIPTDIR)
PATH=$stage2_dir/bin:$PATH storj-sim -x --host $STORJ_NETWORK_HOST4 network test bash "$SCRIPTDIR"/test-backwards.sh download

# TODO write our own script instead of using test-backwards.sh. This script should also print out all satellite/storagenode/uplink versions using the binaries for sanity