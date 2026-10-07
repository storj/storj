#!/usr/bin/env bash

set -ueo pipefail
set -x

TMP=$(mktemp -d -t tmp.XXXXXXXXXX)

cleanup(){
    ret=$?
    echo "EXIT STATUS: $ret"
    git worktree prune
    rm -rf "$TMP"
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

# version_ge returns true if version $1 is greater than or equal to $2
version_ge(){
    [ "$( ( echo "$1"; echo "$2" ) | sort -V | head -n 1 )" = "$2" ]
}

# set this var to anything else than `jenkins` to run tests locally
RUN_TYPE=${RUN_TYPE:-"jenkins"}

# set peers' versions
# in stage 1: satellite and storagenode use latest release version, uplink uses all 3 highest point release from all major releases plus versions from $IMPORTANT_VERSIONS
# in stage 2: satellite core uses latest release version and satellite api uses main. Storage nodes are split into half on latest release version and half on main. Uplink uses the all versions from stage 1 plus main
# stable-versions.txt: first stable version, next 2 versions representative for pre metainfo refactoring, other represent current rclone, duplicati etc.
IMPORTANT_VERSIONS=$(cat "$( dirname "${BASH_SOURCE[0]}" )/stable-versions.txt")

# Note: tags should be fetched before running this script (e.g., in run-postgres.sh or Jenkinsfile)
# to avoid needing network/SSH access from within the container.
# Disable remote access by removing the remote URL - all required refs should already be local.
git remote set-url origin /dev/null 2>/dev/null || true
git fetch --tags 2>/dev/null || true
major_release_tags=$(
    git tag -l --sort -version:refname 'v*' |                        # get the release tag list
    grep -v rc |                                                     # remove release candidates
    sort -n -k2,2 -t'.' --unique |                                   # only keep the largest patch version
    sort -V |                                                        # resort based using "version sort"
    tail -n 3                                                        # kepep only last 3 releases
)
major_release_tags=$(echo $IMPORTANT_VERSIONS $major_release_tags )
current_release_version=$(echo $major_release_tags | xargs -n 1 | tail -1)
stage1_sat_version=$current_release_version
stage1_uplink_versions=$major_release_tags
stage1_storagenode_versions=$(populate_sno_versions $current_release_version 10)
stage2_sat_version="main"
stage2_uplink_versions=$major_release_tags\ "main"
stage2_storagenode_versions=$(populate_sno_versions $current_release_version 5)\ $(populate_sno_versions "main" 5)

echo "stage1_sat_version" $stage1_sat_version
echo "stage1_uplink_versions" $stage1_uplink_versions
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

scriptdir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

# mirroring install-sim from the Makefile since it won't work on private Jenkins
install_sim(){
    local work_dir="$1"
    local bin_dir="$2"
    mkdir -p ${bin_dir}

    # build from the version's worktree, not from the checkout under test
    pushd ${work_dir}
    go build -race -o ${bin_dir}/storagenode storj.io/storj/cmd/storagenode 2>&1
    go build -race -o ${bin_dir}/satellite storj.io/storj/cmd/satellite 2>&1
    go build -race -o ${bin_dir}/storj-sim storj.io/storj/cmd/storj-sim 2>&1
    go build -race -o ${bin_dir}/versioncontrol storj.io/storj/cmd/versioncontrol 2>&1

    go build -race -o ${bin_dir}/uplink storj.io/storj/cmd/uplink 2>&1
    go build -race -o ${bin_dir}/identity storj.io/storj/cmd/identity 2>&1
    go build -race -o ${bin_dir}/certificates storj.io/storj/cmd/certificates 2>&1

    if [ -d "${work_dir}/cmd/gateway" ]; then
        (cd ${work_dir}/cmd/gateway && go build -race -o ${bin_dir}/gateway storj.io/storj/cmd/gateway 2>&1)
    else
        GOBIN=${bin_dir} go install -race storj.io/gateway@latest
    fi
    if [ -d "${work_dir}/cmd/multinode" ]; then
        # multinode versions that are below c08ca361d83b252da8ba466896f23fdc6dddc1d9 throws on run if UI was not build
        go build -race -o ${bin_dir}/multinode storj.io/storj/cmd/multinode 2>&1
    fi

    if [ -d "${work_dir}/cmd/jobq" ]; then
        go build -race -o ${bin_dir}/jobq storj.io/storj/cmd/jobq 2>&1
    fi
    popd
}

setup_stage(){
    local test_dir=$1
    local sat_version=$2
    local stage_sn_versions=$3

    echo "Satellite version: ${sat_version}"
    echo "Storagenode versions: ${stage_sn_versions}"

    local src_sat_version_dir=$(version_dir ${sat_version})

    # if the new storj-sim manages a jobq peer that the previous one didn't,
    # seed the jobq config/identity and binary so subsequent storj-sim network
    # commands against test_dir can find them. This must happen before the
    # network env lookups below, since some use the new storj-sim against test_dir.
    if [[ -d "${src_sat_version_dir}/local-network/jobq" && ! -d "${test_dir}/local-network/jobq" ]]; then
        cp -r "${src_sat_version_dir}/local-network/jobq" "${test_dir}/local-network/jobq"
        if [[ -f "${src_sat_version_dir}/bin/jobq" ]]; then
            ln -f "${src_sat_version_dir}/bin/jobq" "${test_dir}/bin/jobq"
        fi
    fi

    PATH=$src_sat_version_dir/bin:$PATH src_sat_cfg_dir=$(storj-sim network env --config-dir=${src_sat_version_dir}/local-network/ SATELLITE_0_DIR)
    PATH=$test_dir/bin:$PATH dest_sat_cfg_dir=$(storj-sim network env --config-dir=${test_dir}/local-network/ SATELLITE_0_DIR)

    # ln binary and copy config.yaml for desired version
    ln -f $(version_dir ${sat_version})/bin/storj-sim $test_dir/bin/storj-sim
    ln -f $src_sat_version_dir/bin/satellite $dest_sat_cfg_dir/satellite
    cp $src_sat_cfg_dir/config.yaml $dest_sat_cfg_dir
    replace_in_file "${src_sat_version_dir}" "${test_dir}" "${dest_sat_cfg_dir}/config.yaml"
    replace_in_file "\# console.usage-limits.default-bandwidth-limit:.*" "console.usage-limits.default-bandwidth-limit: 500GB" "${dest_sat_cfg_dir}/config.yaml"
    replace_in_file "\# console.usage-limits.default-storage-limit:.*" "console.usage-limits.default-storage-limit: 500GB" "${dest_sat_cfg_dir}/config.yaml"

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
        replace_in_file  "${src_sat_nodeid}" "${dest_sat_nodeid}" "${dest_sn_cfg_dir}/config.yaml"

        let counter+=1
    done
}

# Set up each environment
unique_versions=$(find_unique_versions "$stage1_sat_version" "$stage1_uplink_versions" "$stage1_storagenode_versions" "$stage2_sat_version" "$stage2_uplink_versions" "$stage2_storagenode_versions")

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

# BIN_CACHE_DIR stores built binaries for release tags across runs, since tags never change.
# main is always rebuilt. Leave empty to disable caching.
BIN_CACHE_DIR=${BIN_CACHE_DIR:-""}
if [ -n "${BIN_CACHE_DIR}" ]; then
    mkdir -p "${BIN_CACHE_DIR}"
fi

# go_for_version prints the go command used to build the uplink for a version.
go_for_version(){
    local version=$1
    if [[ $version = "main" ]] || version_ge "$version" "v1.64.0"; then
        echo go
    else
        echo go1.16.15
    fi
}

# bin_cache_dir prints the cache directory for a version, or nothing when it shouldn't be cached.
bin_cache_dir(){
    local version=$1
    if [[ -n "${BIN_CACHE_DIR}" && $version != "main" ]]; then
        # keyed on the toolchain only, so gateway@latest for the current release stays as built until the next release.
        # go1.16.15 may not be installed, so its name doubles as the version
        local goversion=$(go_for_version $version)
        if [[ $goversion = go ]]; then
            goversion=$(go env GOVERSION)
        fi
        echo "${BIN_CACHE_DIR}/${version}-${goversion}-$(go env GOARCH)"
    fi
}

# download_bin_cache adds the missing stable versions to BIN_CACHE_DIR from the asset
# uploaded by `make test/uplink-versions/publish-binaries`.
download_bin_cache(){
    local archive=${TMP}/uplink-versions-binaries.tar.xz extract_dir=${TMP}/bin-cache-download entry
    if [ ! -s "${scriptdir}/stable-versions.sha256" ]; then
        echo "No stable-versions.sha256, the binaries haven't been published."
        return 1
    fi
    curl -fsSL -o "$archive" "https://github.com/storj/storj/releases/download/uplink-versions-binaries/uplink-versions-binaries.tar.xz" || return 1
    # the release asset can be replaced, so only trust the checksum committed to the repository
    if [[ "$(shasum -a 256 < "$archive" | cut -d' ' -f1)" != "$(cat "${scriptdir}/stable-versions.sha256")" ]]; then
        echo "Checksum of uplink-versions-binaries.tar.xz doesn't match stable-versions.sha256."
        return 1
    fi
    mkdir -p "$extract_dir"
    tar -xJf "$archive" -C "$extract_dir" || return 1
    # move complete entries only, so an interrupted download never leaves a partial cache entry
    for entry in "$extract_dir"/*; do
        if [[ -d "$entry" && ! -e "${BIN_CACHE_DIR}/$(basename "$entry")" ]]; then
            mv "$entry" "${BIN_CACHE_DIR}/"
        fi
    done
    rm -rf "$extract_dir" "$archive"
}

if [ -n "${BIN_CACHE_DIR}" ]; then
    for version in ${IMPORTANT_VERSIONS}; do
        if [ ! -d "$(bin_cache_dir $version)" ]; then
            download_bin_cache || echo "Downloading stable binaries failed, building them instead."
            break
        fi
    done
fi

# go1.16.15 is only needed to build the uplinks missing from the cache
for version in ${unique_versions}; do
    cache_dir=$(bin_cache_dir $version)
    if [[ $(go_for_version $version) = go1.16.15 && ! ( -n "$cache_dir" && -d "$cache_dir" ) ]]; then
        if ! command -v go1.16.15 &> /dev/null; then
            echo "Installing old Go version"
            go install golang.org/dl/go1.16.15@latest && go1.16.15 download
        fi
        break
    fi
done

echo "Setting up environments for versions" ${unique_versions}

# create a result file for each child process' exit code
exit_statuses_dir=${TMP}/exit_statuses
mkdir ${exit_statuses_dir}
write_exit_status(){
    # we need to capture the exit status first so it won't be overwritten
    local exit_status=$?
    local version=$1
    echo $exit_status > ${exit_statuses_dir}/${version} # write exit code to a file named with the current installing version
    echo "installation for ${version} exited with status $exit_status"
}
# clean up git worktree
git worktree prune
for version in ${unique_versions}; do
    # run in parallel
    (
        trap "write_exit_status ${version}" EXIT
        dir=$(version_dir ${version})
        bin_dir=${dir}/bin

        echo -e "\nAdding worktree for ${version} in ${dir}."
        if [[ $version = "main" ]]
        then
            git worktree add -f "$dir" "origin/main"
        else
            git worktree add -f "$dir" "${version}"
        fi

        rm -f ${dir}/internal/version/release.go
        if [ -d "${dir}/private/version/release.go" ]; then
            # clear out release information
            cat > ${dir}/private/version/release.go <<-EOF
		// Copyright (C) 2020 Storj Labs, Inc.
		// See LICENSE for copying information.
		package version
		EOF
        fi

        cache_dir=$(bin_cache_dir ${version})
        if [[ -n "$cache_dir" && -d "$cache_dir" ]]; then
            echo "Restoring binaries for ${version} from ${cache_dir}."
            mkdir -p ${bin_dir}
            cp -a "${cache_dir}/." ${bin_dir}/
            if [[ -f ${bin_dir}/storj-sim ]]; then
                # storj-sim finds web assets and email templates relative to its source path, which was
                # the worktree of the run that filled the cache, so rebuild it from this worktree.
                (cd ${dir} && go build -race -o ${bin_dir}/storj-sim storj.io/storj/cmd/storj-sim 2>&1)
            fi
        elif [[ $version = $current_release_version || $version = "main" ]]; then
            echo "Installing storj-sim for ${version} in ${dir}."
            install_sim ${dir} ${bin_dir}
            echo "finished installing"
        else
            echo "Installing uplink for ${version} in ${dir}."
            pushd ${dir}
            mkdir -p ${bin_dir}

            if [[ $(go_for_version $version) = go1.16.15 ]]; then
                # without -race and debug info to match Dockerfile.binaries
                go1.16.15 build -trimpath -ldflags=-s -o ${bin_dir}/uplink storj.io/storj/cmd/uplink 2>&1
            else
                go build -race -o ${bin_dir}/uplink storj.io/storj/cmd/uplink 2>&1
            fi

            popd
        fi
        if [[ -n "$cache_dir" && ! -d "$cache_dir" ]]; then
            # copy to a temp dir first so an interrupted copy never looks like a valid cache entry
            cp -a ${bin_dir} "${cache_dir}.tmp.$$"
            mv "${cache_dir}.tmp.$$" "${cache_dir}"
        fi

        if [[ $version = $current_release_version || $version = "main" ]]
        then
            echo "Setting up storj-sim for ${version}. Bin: ${bin_dir}, Config: ${dir}/local-network"
            PATH=${bin_dir}:$PATH storj-sim -x --host="${STORJ_NETWORK_HOST4}" --postgres="${STORJ_SIM_POSTGRES}" --config-dir "${dir}/local-network" network setup
            echo "Finished setting up. ${dir}/local-network:" $(ls ${dir}/local-network)
            echo "Binary shasums:"
            shasum ${bin_dir}/satellite
            shasum ${bin_dir}/storagenode
            shasum ${bin_dir}/uplink
            shasum ${bin_dir}/gateway
        else
            echo "Finished installing. ${bin_dir}:" $(ls ${bin_dir})
            echo "Binary shasums:"
            shasum ${bin_dir}/uplink
        fi
    ) &
done

wait # wait for all child processes to finish
# iterate through those result files to check their exit code
# if there's any exit code that's non-zero, exit the test
grep -qvwr "0" ${exit_statuses_dir} && exit 1

# Use stage 1 satellite version as the starting state. Create a cp of that
# version folder so we don't worry about dirty states. Then copy/link/mv
# appropriate resources into that folder to ensure we have correct versions.
test_dir=$(version_dir "test_dir")
cp -r $(version_dir ${stage1_sat_version}) ${test_dir}
echo -e "\nSetting up stage 1 in ${test_dir}"
setup_stage "${test_dir}" "${stage1_sat_version}" "${stage1_storagenode_versions}"
update_access_script_path="$(version_dir "main")/testsuite/update-access/main.go"

# Uploading files to the network using the latest release version for each uplink version
for ul_version in ${stage1_uplink_versions}; do
    echo "Stage 1 Uplink version: ${ul_version}"
    src_ul_version_dir=$(version_dir ${ul_version})
    ln -f ${src_ul_version_dir}/bin/uplink $test_dir/bin/uplink
    PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${scriptdir}/steps.sh" "${test_dir}/local-network" "upload" "${ul_version}" "$update_access_script_path"
done
# Remove current uplink config to regenerate uplink config for older uplink version
rm -rf "${test_dir}/local-network/uplink"

echo -e "\nSetting up stage 2 in ${test_dir}"
setup_stage "${test_dir}" "${stage2_sat_version}" "${stage2_storagenode_versions}"
echo -e "\nRunning stage 2."

# Downloading every file uploaded in stage 1 from the network using the latest commit from main branch for each uplink version
for ul_version in ${stage2_uplink_versions}; do
    echo "Stage 2 Uplink version: ${ul_version}"
    src_ul_version_dir=$(version_dir ${ul_version})
    ln -f ${src_ul_version_dir}/bin/uplink $test_dir/bin/uplink
    PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${scriptdir}/steps.sh" "${test_dir}/local-network" "download" "${ul_version}" "$update_access_script_path" "${stage1_uplink_versions}"
done


echo -e "\nCleaning up."
PATH=$test_dir/bin:$PATH storj-sim -x --host "${STORJ_NETWORK_HOST4}" --config-dir "${test_dir}/local-network" network test bash "${scriptdir}/steps.sh" "${test_dir}/local-network" "cleanup" "${stage1_uplink_versions}" ""
