#!/usr/bin/env bash
set -o errexit

trap "echo ERROR: exiting due to error; exit" ERR
trap "exit" INT TERM
trap "kill 0" EXIT

. $(dirname $0)/utils.sh

user_id="user@example.com"
signer_address="127.0.0.1:8888"
difficulty=16

cleanup() {
    if [[ ! -z ${bg+x} ]]; then
        kill ${bg}
    fi

    dirs="$tmp $tmp_build_dir"
    for dir in ${dirs}; do
        if [[ ! -z ${dir+x} ]]; then
            rm -rf ${dir}
        fi
    done
}
if [[ ${TRAVIS} == true ]]; then
    declare_cmds storagenode certificates
else
    temp_build storagenode certificates
fi
tmp=$(mktemp -d)
trap "cleanup" EXIT


certificates_dir=${tmp}/cert-signing
storagenode_dir=${tmp}/storagenode

# TODO: create separate signer CA and use `--signer.ca` options
#                    --signer.ca.cert-path ${signer_cert} \
#                    --signer.ca.key-path ${signer_key} \

echo "setting up certificate signing server"
$certificates setup --config-dir ${certificates_dir} \
                    --signer.min-difficulty ${difficulty}

echo "creating test authorization"
$certificates auth create --config-dir ${certificates_dir} \
                          1 ${user_id} >/dev/null 2>&1

export_tokens() {
    $certificates auth export --config-dir ${certificates_dir} \
                              --out -
}
token=$(export_tokens 2>&1|cut -d , -f 2|grep -oE "$user_id:\w+")

echo "starting certificate signing server"
$certificates run --config-dir ${certificates_dir} \
                  --server.address ${signer_address} &
bg=$!
sleep 1

echo "setting up storage node"
$storagenode setup --config-dir ${storagenode_dir} \
                   --ca.difficulty ${difficulty} \
                   --signer.address ${signer_address} \
                   --signer.auth-token ${token}

ca_chain_len=$(cat ${storagenode_dir}/ca.cert|grep "BEGIN CERTIFICATE"|wc -l)
ident_chain_len=$(cat ${storagenode_dir}/identity.cert|grep "BEGIN CERTIFICATE"|wc -l)
failures=0
if [[ ! ${ca_chain_len} == 2 ]]; then
    echo "FAIL: incorrect storage node CA chain length; expected: 2; actual: ${ca_chain_len}"
    failures=$((failures+1))
fi
if [[ ! ${ident_chain_len} == 3 ]]; then
    echo "FAIL: incorrect storage node identty chain length; expected: 2; actual: ${ident_chain_len}"
    failures=$((failures+1))
fi

if [[ ${failures} == 0 ]]; then
    echo "SUCCESS: all expectations met!"
fi

exit ${failures}
