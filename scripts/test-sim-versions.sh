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
#trap cleanup EXIT

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

 # TODO make sure the number of storagenode versions matches the number of sns from setup

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

replace_in_file(){
    local src="$1"
    local dest="$2"
    local path=$3
    sed -i "s#${src}#${dest}#g" "${path}"
}

setup_stage(){
    local test_dir=$1
    local sat_version=$2
    local stage_sn_versions=$3
    local stage_ul_version=$4

    echo "setting up with sat, sns, ul"
    echo $sat_version
    echo $stage_sn_versions
    echo $stage_ul_version

    local src_sat_version_dir=$(version_dir ${sat_version})

    PATH=$src_sat_version_dir/bin:$PATH src_sat_cfg_dir=$(storj-sim network env --config-dir=${src_sat_version_dir}/local-network/ SATELLITE_0_DIR)
    PATH=$test_dir/bin:$PATH dest_sat_cfg_dir=$(storj-sim network env --config-dir=${test_dir}/local-network/ SATELLITE_0_DIR)
    echo "src_sat_version_dir" ${src_sat_version_dir}
    echo "test_dir" ${test_dir}
    echo "$src_sat_version_dir/bin" $(ls $src_sat_version_dir/bin)
    echo "$test_dir/bin" $(ls $test_dir/bin)
    echo "src_sat_cfg_dir" ${src_sat_cfg_dir}
    echo "dest_sat_cfg_dir" ${dest_sat_cfg_dir}

    # ln binary and copy config.yaml for desired version
    ln -f $src_sat_version_dir/bin/satellite $dest_sat_cfg_dir/satellite
    cp $src_sat_cfg_dir/config.yaml $dest_sat_cfg_dir
    replace_in_file "${src_sat_version_dir}" "${test_dir}" "${dest_sat_cfg_dir}/config.yaml"

    counter=0
    for sn_version in ${stage_sn_versions}; do
        echo "next sn version" ${sn_version}
        local src_sn_version_dir=$(version_dir ${sn_version})

        PATH=$src_sn_version_dir/bin:$PATH src_sn_cfg_dir=$(storj-sim network env --config-dir=${src_sn_version_dir}/local-network/ STORAGENODE_${counter}_DIR)
        PATH=$test_dir/bin:$PATH dest_sn_cfg_dir=$(storj-sim network env --config-dir=${test_dir}/local-network/ STORAGENODE_${counter}_DIR)

        echo "!!!!!!!!!"
        echo "src_sn_version_dir" ${src_sn_version_dir}
        echo "test_dir" ${test_dir}
        echo "$src_sn_version_dir/bin" $(ls $src_sn_version_dir/bin)
        echo "$test_dir/bin" $(ls $test_dir/bin)
        echo "src_sn_cfg_dir" ${src_sn_cfg_dir}
        echo "dest_sn_cfg_dir" ${dest_sn_cfg_dir}

        dest_sat_nodeid=$(grep "storage.whitelisted-satellites" ${dest_sn_cfg_dir}/config.yaml)
        echo "dest_sat_nodeid" "${dest_sat_nodeid}"
        src_sat_nodeid=$(grep "storage.whitelisted-satellites" "${src_sn_cfg_dir}/config.yaml")

        # ln binary and copy config.yaml for desired version
        ln -f $src_sn_version_dir/bin/storagenode $dest_sn_cfg_dir/storagenode
        cp $src_sn_cfg_dir/config.yaml $dest_sn_cfg_dir

        # update config dir in config.yaml as well as whitelisted satellites in config.yaml
        replace_in_file "${src_sn_version_dir}" "${test_dir}" "${dest_sn_cfg_dir}/config.yaml"
        replace_in_file  "${src_sat_nodeid}" "${dest_sat_nodeid}" "${dest_sn_cfg_dir}/config.yaml"

        let counter+=1
    done

    # use desired uplink binary and config
    src_ul_version_dir=$(version_dir ${stage_ul_version})
    # PATH=$src_ul_version_dir/bin:$PATH src_ul_cfg_dir=$(storj-sim network env --config-dir=${src_ul_version_dir}/local-network/ GATEWAY_0_DIR)
    # PATH=$test_dir/bin:$PATH dest_ul_cfg_dir=$(storj-sim network env --config-dir=${test_dir}/local-network/ GATEWAY_0_DIR)

    # src_ul_scope=$(grep "scope" "${src_ul_cfg_dir}/config.yaml")
    # dest_ul_scope=$(grep "scope" "${dest_ul_cfg_dir}/config.yaml")

    # cp $src_ul_cfg_dir/config.yaml $dest_ul_cfg_dir
    # replace_in_file "${src_ul_version_dir}" "${test_dir}" "${dest_ul_cfg_dir}/config.yaml"
    # replace_in_file "${src_ul_scope}" "${dest_ul_scope}" "${dest_ul_cfg_dir}/config.yaml"
    ln -f $src_ul_version_dir/bin/uplink $test_dir/bin/uplink
}

# Set up each environment
unique_versions=$(find_unique_versions "$stage1_sat_version" "$stage1_uplink_version" "$stage1_storagenode_versions" "$stage2_sat_version" "$stage2_uplink_version" "$stage2_storagenode_versions")

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
    PATH=${bin_dir}:$PATH storj-sim -x --host="${STORJ_NETWORK_HOST4}" --postgres="${STORJ_SIM_POSTGRES}" --config-dir "${dir}/local-network" network setup

    echo "Finished setting up:" $(ls ${dir}/local-network)
done

# Use stage 1 satellite version as the starting state. Create a cp of that
# version folder so we don't worry about dirty states. Then copy/link/mv
# appropriate resources into that folder to ensure we have correct versions.
test_dir=$(version_dir "test_dir")
cp -r $(version_dir ${stage1_sat_version}) ${test_dir}
setup_stage "${test_dir}" "${stage1_sat_version}" "${stage1_storagenode_versions}" "${stage1_uplink_version}"

# TODO: Run tests here
scriptdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${scriptdir}/test-versions.sh" "${test_dir}/local-network" "upload"

setup_stage "${test_dir}" "${stage2_sat_version}" "${stage2_storagenode_versions}" "${stage2_uplink_version}"

# TODO: Run stage 2 tests here
PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${scriptdir}/test-versions.sh" "${test_dir}/local-network" "download"
PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${scriptdir}/test-versions.sh" "${test_dir}/local-network" "cleanup"