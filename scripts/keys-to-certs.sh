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

    # Generate CA from key
    cert=${dir}/${label}.cert
    $identity ca new --ca.key-path ${key} \
                     --ca.cert-path ${cert}

    # Get node ID
    id=$($identity ca id --ca.cert-path ${cert} | cut -c 1-10)
    if [[ $? != 0 ]]; then
        exit $?
    fi
    work_dir=${dir}/${id}

    echo "  - ${key}"

    mkdir ${work_dir}
    trap "temp_cleanup ${work_dir}" EXIT ERR INT

    ca_key_path=${work_dir}/ca.key
    ca_cert_path=${work_dir}/ca.cert
    cp ${key} ${ca_key_path}
    mv ${cert} ${ca_cert_path}

    # Generate new identity from CA
    $identity id new --ca.key-path ${ca_key_path} \
                     --ca.cert-path ${ca_cert_path} \
                     --identity.key-path ${work_dir}/identity.key \
                     --identity.cert-path ${work_dir}/identity.cert

    # Create tarball
    tar -C ${work_dir}/.. -cJf ${label}.txz ${id}

    # Remove working directory
    rm -rf ${work_dir}
done
echo "done"