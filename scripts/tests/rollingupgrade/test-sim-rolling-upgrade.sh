#!/usr/bin/env bash

# This file is the entrypoint for the rolling upgrade test.
# Description of test:
# Stage 1:
#  * Set up a storj-sim network on the latest point release (all storagenodes and satellite on latest point release).
#  * If the current commit is a release tag use the previous release instead. Exclude -rc release tags.
#  * Check out the latest point release of the uplink.
#  * (test-versions.sh upload) - Upload an inline, remote, and multisegment file to the network with the selected uplink.
# Stage 2:
#  * Upgrade the satellite to current commit. Run an "old" satellite api server on the latest point release (port 30000).
#  * Keep half of the storagenodes on the latest point release. Upgrade the other half to master.
#  * Point half of the storagenodes to the old satellite api (port 30000). Keep the other half on the new satellite api (port 10000).
#  * Check out the master version of the uplink.
#  * (test-rolling-upgrade.sh) - Download the inline, remote, and multisegment file from the network using the master uplink and the new satellite api.
#  * (test-rolling-upgrade.sh) - Download the inline, remote, and multisegment file from the network using the master uplink and the old satellite api.
#  * (test-rolling-upgrade-final-upload.sh) - Upload an inline, remote, and multisegment file to the network using the master uplink and the new satellite api.
#  * (test-rolling-upgrade-final-upload.sh) - Upload an inline, remote, and multisegment file to the network using the master uplink and the old satellite api.
#  * (test-rolling-upgrade-final-upload.sh) - Download the six inline, remote, and multisegment files from the previous two steps using the master uplink and new satellite api.
#  * (test-rolling-upgrade-final-upload.sh) - Download the six inline, remote, and multisegment files from the previous two steps using the master uplink and old satellite api.

set -ueo pipefail
set +x

old_api_pid=-1
cleanup(){
    ret=$?
    echo "EXIT STATUS: $ret"
    git worktree prune
    rm -rf "$TMP"
    if [[ $old_api_pid != -1 ]]
    then
        kill -2 $old_api_pid
    fi
    echo "cleaned up test successfully"
    exit "$ret"
}
trap cleanup EXIT

# set storagenode versions to use desired storagenode binary
populate_sno_versions(){
    local version=$1
    local number_of_nodes=$2
    seq $number_of_nodes | xargs -n1 -I{} echo $version
}

# set peers' versions
# in stage 1: satellite, uplink, and storagenode use latest release version
# in stage 2: satellite core uses latest release version and satellite api uses master. Storage nodes are split into half on latest release version and half on master. Uplink uses the latest release version plus master
git fetch --tags
current_commit=$(git rev-parse HEAD)
current_release_version=$(git describe --tags $current_commit | cut -d '.' -f 1-2)
previous_release_version=$(git describe --tags `git rev-list --exclude='*rc*' --exclude=$current_release_version* --tags --max-count=1`)
stage1_sat_version=$previous_release_version
stage1_uplink_version=$previous_release_version
stage1_storagenode_versions=$(populate_sno_versions $previous_release_version 10)
stage2_sat_version=$current_commit
stage2_uplink_versions=$previous_release_version\ $current_commit
stage2_storagenode_versions=$(populate_sno_versions $previous_release_version 5)\ $(populate_sno_versions $current_commit 5)

echo "stage1_sat_version" $stage1_sat_version
echo "stage1_uplink_version" $stage1_uplink_version
echo "stage1_storagenode_versions" $stage1_storagenode_versions
echo "stage2_sat_version" $stage2_sat_version
echo "stage2_uplink_versions" $stage2_uplink_versions
echo "stage2_storagenode_versions" $stage2_storagenode_versions

TMP=$(mktemp -d -t tmp.XXXXXXXXXX)

find_unique_versions(){
    echo "$*" | tr " " "\n" | sort | uniq
}

version_dir(){
    echo "${TMP}/${1}"
}

replace_in_file(){
    local src="$1"
    local dest="$2"
    local path=$3
    case "$OSTYPE" in
    darwin*)
        sed -i '' "s#${src}#${dest}#g" "${path}" ;;
    *)
        sed -i "s#${src}#${dest}#g" "${path}" ;;
    esac
}

# mirroring install-sim from the Makefile since it won't work on private Jenkins
install_sim(){
    local work_dir="$1"
    local bin_dir="$2"
    mkdir -p ${bin_dir}

    go build -race -v -tags=grpc -o ${bin_dir}/storagenode-grpc storj.io/storj/cmd/storagenode >/dev/null 2>&1
    go build -race -v -tags=drpc -o ${bin_dir}/storagenode-drpc storj.io/storj/cmd/storagenode >/dev/null 2>&1
    go build -race -v -tags=grpc -o ${bin_dir}/satellite-grpc storj.io/storj/cmd/satellite >/dev/null 2>&1
    go build -race -v -tags=drpc -o ${bin_dir}/satellite-drpc storj.io/storj/cmd/satellite >/dev/null 2>&1

    go build -race -v -o ${bin_dir}/storagenode storj.io/storj/cmd/storagenode >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/satellite storj.io/storj/cmd/satellite >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/storj-sim storj.io/storj/cmd/storj-sim >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/versioncontrol storj.io/storj/cmd/versioncontrol >/dev/null 2>&1

    go build -race -v -o ${bin_dir}/uplink storj.io/storj/cmd/uplink >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/identity storj.io/storj/cmd/identity >/dev/null 2>&1
    go build -race -v -o ${bin_dir}/certificates storj.io/storj/cmd/certificates >/dev/null 2>&1

    if [ -d "${work_dir}/cmd/gateway" ]; then
        pushd ${work_dir}/cmd/gateway
            go build -race -v -o ${bin_dir}/gateway storj.io/storj/cmd/gateway >/dev/null 2>&1
        popd
    else
        rm -rf .build/gateway-tmp
        mkdir -p .build/gateway-tmp
        pushd .build/gateway-tmp
            go mod init gatewaybuild && GOBIN=${bin_dir} GO111MODULE=on go get storj.io/gateway@v1.0.0-rc.8
        popd
    fi
}

setup_stage(){
    local test_dir=$1
    local sat_version=$2
    local stage_sn_versions=$3
    local stage=$4

    echo "Satellite version: ${sat_version}"
    echo "Storagenode versions: ${stage_sn_versions}"
    echo "Stage: ${stage}"

    local src_sat_version_dir=$(version_dir ${sat_version})

    PATH=$src_sat_version_dir/bin:$PATH src_sat_cfg_dir=$(storj-sim network env --config-dir=${src_sat_version_dir}/local-network/ SATELLITE_0_DIR)
    PATH=$test_dir/bin:$PATH dest_sat_cfg_dir=$(storj-sim network env --config-dir=${test_dir}/local-network/ SATELLITE_0_DIR)

    # if stage 2, move old satellite binary to old_satellite
    if [[ $stage == "2" ]]
    then
        mv $dest_sat_cfg_dir/satellite $dest_sat_cfg_dir/old_satellite
    fi
    # ln binary and copy config.yaml for desired version
    ln -f $src_sat_version_dir/bin/satellite $dest_sat_cfg_dir/satellite
    cp $src_sat_cfg_dir/config.yaml $dest_sat_cfg_dir
    replace_in_file "${src_sat_version_dir}" "${test_dir}" "${dest_sat_cfg_dir}/config.yaml"

    counter=0
    for sn_version in ${stage_sn_versions}; do
        local src_sn_version_dir=$(version_dir ${sn_version})

        PATH=$src_sn_version_dir/bin:$PATH src_sn_cfg_dir=$(storj-sim network env --config-dir=${src_sn_version_dir}/local-network/ STORAGENODE_${counter}_DIR)
        PATH=$test_dir/bin:$PATH dest_sn_cfg_dir=$(storj-sim network env --config-dir=${test_dir}/local-network/ STORAGENODE_${counter}_DIR)

        dest_sat_nodeid=$(grep "storage2.trust.source" ${dest_sn_cfg_dir}/config.yaml || grep "storage.whitelisted-satellites" ${dest_sn_cfg_dir}/config.yaml)
        dest_sat_nodeid=$(echo $dest_sat_nodeid | grep -o ": .*@")
        src_sat_nodeid=$(grep "storage2.trust.source" ${src_sn_cfg_dir}/config.yaml || grep "storage.whitelisted-satellites" ${src_sn_cfg_dir}/config.yaml)
        src_sat_nodeid=$(echo $src_sat_nodeid | grep -o ": .*@")

        # ln binary and copy config.yaml for desired version
        ln -f $src_sn_version_dir/bin/storagenode $dest_sn_cfg_dir/storagenode
        cp $src_sn_cfg_dir/config.yaml $dest_sn_cfg_dir

        # update config dir in config.yaml as well as whitelisted satellites in config.yaml
        replace_in_file "${src_sn_version_dir}" "${test_dir}" "${dest_sn_cfg_dir}/config.yaml"
        replace_in_file "${src_sat_nodeid}" "${dest_sat_nodeid}" "${dest_sn_cfg_dir}/config.yaml"

        # if stage 2, point half of the nodes to the old satellite api
        if [[ $stage = "2" && $(($counter % 2)) = 0 ]]
        then
            replace_in_file "127.0.0.1:10000" "127.0.0.1:30000" "${dest_sn_cfg_dir}/config.yaml"
        fi

        let counter+=1
    done
}

# Set up each environment
unique_versions=$(find_unique_versions "$stage1_sat_version" "$stage1_uplink_version" "$stage1_storagenode_versions" "$stage2_sat_version" "$stage2_uplink_versions" "$stage2_storagenode_versions")

STORJ_NETWORK_HOST4=${STORJ_NETWORK_HOST4:-127.0.0.1}
STORJ_SIM_POSTGRES=${STORJ_SIM_POSTGRES:-""}
STORJ_SIM_REDIS=${STORJ_SIM_REDIS:-""}

if [ -z ${STORJ_SIM_POSTGRES} ]; then
    echo "STORJ_SIM_POSTGRES is required for the satellite DB. Example: STORJ_SIM_POSTGRES=postgres://[user]:[pass]@[host]/[db]?sslmode=disable"
    exit 1
fi

if [ -z ${STORJ_SIM_REDIS} ]; then
    echo "STORJ_SIM_REDIS is required for the satellite DB. Example: STORJ_SIM_REDIS=127.0.0.1:[port]"
    exit 1
fi

echo "Setting up environments for versions" ${unique_versions}

scriptdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# Get latest release tags and clean up git worktree
git worktree prune
for version in ${unique_versions}; do
    dir=$(version_dir ${version})
    bin_dir=${dir}/bin

    echo -e "\nAdding worktree for ${version} in ${dir}."
    git worktree add -f "$dir" "${version}"
    rm -f ${dir}/private/version/release.go
    rm -f ${dir}/internal/version/release.go
    if [[ $version = $previous_release_version || $version = $current_commit ]]
    then
        echo "Installing storj-sim for ${version} in ${dir}."
        pushd ${dir}
        # uncomment for Jenkins testing:
        install_sim ${dir} ${bin_dir}
        # uncomment for local testing:
        # GOBIN=${bin_dir} make -C "${dir}" install-sim > /dev/null 2>&1
        echo "finished installing"
        popd
        echo "Setting up storj-sim for ${version}. Bin: ${bin_dir}, Config: ${dir}/local-network"
        PATH=${bin_dir}:$PATH storj-sim -x --host="${STORJ_NETWORK_HOST4}" --postgres="${STORJ_SIM_POSTGRES}" --config-dir "${dir}/local-network" network setup > /dev/null 2>&1
        echo "Finished setting up. ${dir}/local-network:" $(ls ${dir}/local-network)
        echo "Binary shasums:"
        shasum ${bin_dir}/satellite
        shasum ${bin_dir}/storagenode
        shasum ${bin_dir}/uplink
        shasum ${bin_dir}/gateway
    else
        echo "Installing uplink for ${version} in ${dir}."
        pushd ${dir}
        mkdir -p ${bin_dir}
        # uncomment for Jenkins testing:
        go install -race -v -o ${bin_dir}/uplink storj.io/storj/cmd/uplink >/dev/null 2>&1
        # uncomment for local testing:
        # GOBIN=${bin_dir} go install -race -v storj.io/storj/cmd/uplink > /dev/null 2>&1
        popd
        echo "Finished installing. ${bin_dir}:" $(ls ${bin_dir})
        echo "Binary shasums:"
        shasum ${bin_dir}/uplink
    fi
done

# Use stage 1 satellite version as the starting state. Create a cp of that
# version folder so we don't worry about dirty states. Then copy/link/mv
# appropriate resources into that folder to ensure we have correct versions.
test_dir=$(version_dir "test_dir")
cp -r $(version_dir ${stage1_sat_version}) ${test_dir}
echo -e "\nSetting up stage 1 in ${test_dir}"
test_versions_path="$( dirname "${scriptdir}" )/testversions/test-versions.sh"
setup_stage "${test_dir}" "${stage1_sat_version}" "${stage1_storagenode_versions}" "1"
update_access_script_path="$(version_dir $current_commit)/scripts/update-access.go"

# Uploading files to the network using the latest release version
echo "Stage 1 uplink version: ${stage1_uplink_version}"
src_ul_version_dir=$(version_dir ${stage1_uplink_version})
ln -f ${src_ul_version_dir}/bin/uplink $test_dir/bin/uplink
# use test-versions.sh instead of test-rolling-upgrade.sh for upload step, since the setup for the two tests should be identical
PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${test_versions_path}" "${test_dir}/local-network" "upload" "${stage1_uplink_version}" "$update_access_script_path"

echo -e "\nSetting up stage 2 in ${test_dir}"
setup_stage "${test_dir}" "${stage2_sat_version}" "${stage2_storagenode_versions}" "2"
echo -e "\nRunning stage 2."

# Starting old satellite api in the background
old_api_cmd="${test_dir}/local-network/satellite/0/old_satellite run api --config-dir ${test_dir}/local-network/satellite/0/ --debug.addr 127.0.0.1:30009 --server.address 127.0.0.1:30000 --server.private-address 127.0.0.1:30001 --console.address 127.0.0.1:30002 --marketing.address 127.0.0.1:30003"
nohup $old_api_cmd &
# Storing the background process' PID.
old_api_pid=$!

# Downloading every file uploaded in stage 1 from the network using the latest commit from master branch for each uplink version
for ul_version in ${stage2_uplink_versions}; do
    echo "Stage 2 uplink version: ${ul_version}"
    src_ul_version_dir=$(version_dir ${ul_version})
    ln -f ${src_ul_version_dir}/bin/uplink $test_dir/bin/uplink
    PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${scriptdir}/test-rolling-upgrade.sh" "${test_dir}/local-network"  "${stage1_uplink_version}" "$update_access_script_path"

    if [[ $ul_version == $current_commit ]]
    then
        echo "Running final upload/download test on $current_commit"
        PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${scriptdir}/test-rolling-upgrade-final-upload.sh" "${test_dir}/local-network"
    fi
done

echo -e "\nCleaning up."
PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${test_versions_path}" "${test_dir}/local-network" "cleanup" "${stage1_uplink_version}" ""
