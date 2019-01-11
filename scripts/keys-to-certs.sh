#!/usr/bin/env bash
set -o errexit

. $(dirname $0)/utils.sh

cmd=$(basename $0)
check_help $1 "usage: ${cmd} <key> [, ...]
example: ${cmd} ./keys/*.key
example: ${cmd} ./keys/first.key ./keys/second.key

${cmd} creates a tarball of a directory containing the key and a CA certificate derived from that key, for each key passed. Tarballs (and temp directories) are created as siblings to their respective key input files.

The tarball and directory are named after their respective key file but keys and certificates are (re)named to the first 10 characters of the cert's string-encoded node ID."

temp_build identity
trap "temp_cleanup" EXIT ERR INT

echo "processing:"

for key in $@; do
    dir=$(dirname ${key})
    base=$(basename ${key})
    label=${base%.*}
    ext=${base##*.}
    work_dir=${dir}/${label}

    echo "  - ${key}"

    mkdir ${work_dir}
    trap "temp_cleanup ${work_dir}" EXIT ERR INT

    cp ${key} ${work_dir}
    key_path=${work_dir}/${base}
    cert_path=${work_dir}/${label}.cert

    # Generate certificate from key
    $identity ca new --ca.key-path ${key_path} \
                     --ca.cert-path ${cert_path}

    # Get node ID
    id=$($identity ca id --ca.cert-path ${cert_path} | cut -c 1-10)
    if [[ $? != 0 ]]; then
        exit $?
    fi

    # Rename key and cert
    mv ${key_path} ${work_dir}/${id}.${ext}
    mv ${cert_path} ${work_dir}/${id}.cert

    # Create tarball
    tar -C ${work_dir}/.. -cJf ${work_dir}.txz ${label}

    # Remove working directory
    rm -rf ${work_dir}
done
echo "done"