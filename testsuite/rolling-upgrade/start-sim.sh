#!/usr/bin/env bash

# This file is the entrypoint for the rolling upgrade test.
# Description of test:
# Stage 1:
#  * Set up a storj-sim network on the latest point release (all storagenodes and satellite on latest point release).
#  * If the current commit is a release tag use the previous release instead. Exclude -rc release tags.
#  * Check out the latest point release of the uplink.
#  * (uplink-versions/steps.sh upload) - Upload an inline, remote, and multisegment file to the network with the selected uplink.
# Stage 2:
#  * Upgrade the satellite to current commit. Run an "old" satellite api server on the latest point release (port 30000).
#  * Keep half of the storagenodes on the latest point release. Upgrade the other half to main.
#  * Point half of the storagenodes to the old satellite api (port 30000). Keep the other half on the new satellite api (port 10000).
#  * Check out the main version of the uplink.
#  * (step-1.sh) - Download the inline, remote, and multisegment file from the network using the main uplink and the new satellite api.
#  * (step-1.sh) - Download the inline, remote, and multisegment file from the network using the main uplink and the old satellite api.
#  * (step-2.sh) - Upload an inline, remote, and multisegment file to the network using the main uplink and the new satellite api.
#  * (step-2.sh) - Upload an inline, remote, and multisegment file to the network using the main uplink and the old satellite api.
#  * (step-2.sh) - Download the six inline, remote, and multisegment files from the previous two steps using the main uplink and new satellite api.
#  * (step-2.sh) - Download the six inline, remote, and multisegment files from the previous two steps using the main uplink and old satellite api.

set -ueo pipefail
set +x

TMP=$(mktemp -d -t tmp.XXXXXXXXXX)

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

SCRIPTDIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# set peers' versions
# in stage 1: satellite, uplink, and storagenode use latest release version
# in stage 2: satellite core uses latest release version and satellite api uses main. Storage nodes are split into half on latest release version and half on main. Uplink uses the latest release version plus main
BRANCH_NAME=${BRANCH_NAME:-""}
git fetch --tags
# if it's running on a release branch, we will set the stage 1 version to be the latest previous major release
# if it's running on main, we will set the stage 1 version to be the current release version
current_commit=$(git rev-parse HEAD)
# uses { head -1; cat >/dev/null; } to ensure all output from grep is consumed and pipe won't fail
stage1_release_version=$(git tag -l --sort -version:refname | grep -v rc | { head -1; cat >/dev/null; } )
if [[ $BRANCH_NAME = v* ]]; then
    stage1_release_version=$($SCRIPTDIR/../find-previous-release.sh --major)
fi
stage1_sat_version=$stage1_release_version
stage1_uplink_version=$stage1_release_version
stage1_storagenode_versions=$(populate_sno_versions $stage1_release_version 10)
stage2_sat_version=$current_commit
stage2_uplink_versions=$stage1_release_version\ $current_commit
stage2_storagenode_versions=$(populate_sno_versions $stage1_release_version 5)\ $(populate_sno_versions $current_commit 5)

echo "stage1_sat_version" $stage1_sat_version
echo "stage1_uplink_version" $stage1_uplink_version
echo "stage1_storagenode_versions" $stage1_storagenode_versions
echo "stage2_sat_version" $stage2_sat_version
echo "stage2_uplink_versions" $stage2_uplink_versions
echo "stage2_storagenode_versions" $stage2_storagenode_versions

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

    go build -race -o ${bin_dir}/storagenode storj.io/storj/cmd/storagenode 2>&1
    go build -race -o ${bin_dir}/satellite storj.io/storj/cmd/satellite 2>&1
    go build -race -o ${bin_dir}/storj-sim storj.io/storj/cmd/storj-sim 2>&1
    go build -race -o ${bin_dir}/versioncontrol storj.io/storj/cmd/versioncontrol 2>&1

    go build -race -o ${bin_dir}/uplink storj.io/storj/cmd/uplink 2>&1
    go build -race -o ${bin_dir}/identity storj.io/storj/cmd/identity 2>&1
    go build -race -o ${bin_dir}/certificates storj.io/storj/cmd/certificates 2>&1

    if [ -d "${work_dir}/cmd/gateway" ]; then
        pushd ${work_dir}/cmd/gateway
            go build -race -o ${bin_dir}/gateway storj.io/storj/cmd/gateway 2>&1
        popd
    else
        GOBIN=${bin_dir} go install -race storj.io/gateway@latest
    fi

    if [ -d "${work_dir}/cmd/multinode" ]; then
        go build -race -o ${bin_dir}/multinode storj.io/storj/cmd/multinode 2>&1
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
    PATH=$src_sat_version_dir/bin:$PATH src_mnd_cfg_dir=$(storj-sim network env --config-dir=${src_sat_version_dir}/local-network/ MULTINODE_0_DIR)
    PATH=$src_sat_version_dir/bin:$PATH dst_mnd_cfg_dir=$(storj-sim network env --config-dir=${test_dir}/local-network/ MULTINODE_0_DIR)

    # if stage 2, move old satellite binary to old_satellite
    if [[ $stage == "2" ]]
    then
        mv $dest_sat_cfg_dir/satellite $dest_sat_cfg_dir/old_satellite
    fi

    # if using multinode, copy its configuration if none to be backwards compatible
    if [[ "$src_mnd_cfg_dir" != "" && ! -f "$dst_mnd_cfg_dir/config.yaml" ]]; then
        cp -r $src_mnd_cfg_dir/. $dst_mnd_cfg_dir
        # use most recent multinode version to avoid failure when UI was not build
        ln -f $src_sat_version_dir/bin/multinode $test_dir/bin/multinode
    fi

    # ln binary and copy config.yaml for desired version
    ln -f $src_sat_version_dir/bin/storj-sim $test_dir/bin/storj-sim
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

# Get latest release tags and clean up git worktree
git worktree prune
for version in ${unique_versions}; do
    dir=$(version_dir ${version})
    bin_dir=${dir}/bin

    echo -e "\nAdding worktree for ${version} in ${dir}."
    git worktree add -f "$dir" "${version}"
    rm -f ${dir}/internal/version/release.go
    # clear out release information
    cat > ${dir}/private/version/release.go <<-EOF
	// Copyright (C) 2020 Storj Labs, Inc.
	// See LICENSE for copying information.
	package version
	EOF

    if [[ $version = $stage1_release_version || $version = $current_commit ]]
    then
        echo "Installing storj-sim for ${version} in ${dir}."
        pushd ${dir}

        install_sim ${dir} ${bin_dir}

        echo "finished installing"
        popd
        echo "Setting up storj-sim for ${version}. Bin: ${bin_dir}, Config: ${dir}/local-network"
        PATH=${bin_dir}:$PATH storj-sim -x --host="${STORJ_NETWORK_HOST4}" --postgres="${STORJ_SIM_POSTGRES}" --config-dir "${dir}/local-network" network setup > /dev/null 2>&1
        echo "Finished setting up. ${dir}/local-network:" $(ls ${dir}/local-network)
        echo "Binary shasums:"
        shasum ${bin_dir}/storj-sim
        shasum ${bin_dir}/satellite
        shasum ${bin_dir}/storagenode
        shasum ${bin_dir}/uplink
        shasum ${bin_dir}/gateway
    else
        echo "Installing uplink for ${version} in ${dir}."
        pushd ${dir}
        mkdir -p ${bin_dir}

        go build -race -o ${bin_dir}/uplink storj.io/storj/cmd/uplink 2>&1

        popd
        echo "Finished installing. ${bin_dir}:" $(ls ${bin_dir})
        echo "Binary shasums:"
        shasum ${bin_dir}/uplink
    fi
done

# TODO remove this when all tested satellite versions will support compressed baches
export STORJ_COMPRESSED_BATCH=false

# Use stage 1 satellite version as the starting state. Create a cp of that
# version folder so we don't worry about dirty states. Then copy/link/mv
# appropriate resources into that folder to ensure we have correct versions.
test_dir=$(version_dir "test_dir")
cp -r $(version_dir ${stage1_sat_version}) ${test_dir}
echo -e "\nSetting up stage 1 in ${test_dir}"
test_versions_path="$( dirname "${SCRIPTDIR}" )/uplink-versions/steps.sh"
setup_stage "${test_dir}" "${stage1_sat_version}" "${stage1_storagenode_versions}" "1"
update_access_script_path="$(version_dir $current_commit)/testsuite/update-access.go"

# Uploading files to the network using the latest release version
echo "Stage 1 uplink version: ${stage1_uplink_version}"
src_ul_version_dir=$(version_dir ${stage1_uplink_version})
ln -f ${src_ul_version_dir}/bin/uplink $test_dir/bin/uplink
# use uplink-versions/steps.sh instead of ./step-1.sh for upload step, since the setup for the two tests should be identical
PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${test_versions_path}" "${test_dir}/local-network" "upload" "${stage1_uplink_version}" "$update_access_script_path"

echo -e "\nSetting up stage 2 in ${test_dir}"
setup_stage "${test_dir}" "${stage2_sat_version}" "${stage2_storagenode_versions}" "2"
echo -e "\nRunning stage 2."

# For cases where the old satellite predates changeset I0e7e92498c3da768df5b4d5fb213dcd2d4862924,
# adjust all last_net values for future compatibility. this migration step is only necessary for
# satellites which existed before the aforementioned changeset and use dev defaults (to be specific,
# DistinctIP is off). This is a harmless change for any other satellites using dev defaults.
#
# This may need to be done more than once, since in this particular case we will be running an old
# API server the whole time, and it will update nodes with a masked last_net as they check in.
fix_last_nets() {
    $(version_dir ${stage2_sat_version})/bin/satellite --config-dir "${test_dir}/local-network/satellite/0" fix-last-nets
}
if [ ${STORJ_SKIP_FIX_LAST_NETS:-false} == false ]; then
    fix_last_nets
fi

# Starting old satellite api before setting up satellite version ${stage2_sat_version} for stage 2
has_marketing_server=$(echo $stage1_sat_version | awk 'BEGIN{FS="[v.]"} ($2 == 1 && $3 <= 22) || $2 == 0 {print $0}')
if [ "$has_marketing_server" != "" ]; then
    old_api_cmd="${test_dir}/local-network/satellite/0/old_satellite run api --config-dir ${test_dir}/local-network/satellite/0/ --debug.addr 127.0.0.1:30009 --server.address 127.0.0.1:30000 --server.private-address 127.0.0.1:30001 --console.address 127.0.0.1:30002 --marketing.address 127.0.0.1:30003 --marketing.static-dir $(version_dir ${stage1_sat_version})/web/marketing/"
else
    old_api_cmd="${test_dir}/local-network/satellite/0/old_satellite run api --config-dir ${test_dir}/local-network/satellite/0/ --debug.addr 127.0.0.1:30009 --server.address 127.0.0.1:30000 --server.private-address 127.0.0.1:30001 --console.address 127.0.0.1:30002"
fi
nohup $old_api_cmd &
# Storing the background process' PID.
old_api_pid=$!
# Wait until old satellite api is responding to requests to ensure it happens before migration.
storj-sim tool wait-for --retry 50 --interval 100ms  "127.0.0.1:30000"

# Prime the old satellite api. We do this to help catch any issues with DB migrations in regards to statement caches.
echo "Priming old satellite API by uploading, downloading, and deleting objects"
${SCRIPTDIR}/test-previous-satellite.sh "${test_dir}" "$update_access_script_path"

# Downloading every file uploaded in stage 1 from the network using the latest commit from main branch for each uplink version
for ul_version in ${stage2_uplink_versions}; do
    if [ "$ul_version" = "v1.6.3" ]; then
        # TODO: skip v1.6.3 uplink since it doesn't support changing imported access satellite address
        continue
    elif [ "$ul_version" = "v1.6.4" ]; then
        # TODO: skip v1.6.4 uplink since it doesn't support changing imported access satellite address
        continue
    fi
    echo "Stage 2 uplink version: ${ul_version}"
    src_ul_version_dir=$(version_dir ${ul_version})
    ln -f ${src_ul_version_dir}/bin/uplink $test_dir/bin/uplink
    PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${SCRIPTDIR}/step-1.sh" "${test_dir}/local-network"  "${stage1_uplink_version}" "$update_access_script_path"

    if [[ $ul_version == $current_commit ]];then
        echo "Running final upload/download test on $current_commit"
        PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${SCRIPTDIR}/step-2.sh" "${test_dir}/local-network"
    fi
done

# Check that the old satellite api doesn't fail.
echo "Checking old satellite API by uploading, downloading, and deleting objects"
${SCRIPTDIR}/test-previous-satellite.sh "${test_dir}" "$update_access_script_path"

echo -e "\nCleaning up."
PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${test_versions_path}" "${test_dir}/local-network" "cleanup" "${stage1_uplink_version}" ""
