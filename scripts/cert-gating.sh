#!/usr/bin/env bash

basepath=$HOME/.storj/capt
alpha_config=$basepath/config-alpha.yaml
unauthorized_config=$basepath/config-unauthorized.yaml
ca_whitelist=$basepath/ca-alpha-whitelist.cert
ca_count=5
ca_basepath=$basepath/ca-alpha-

ca_i_basepath() {
    echo ${ca_basepath}${1}
}
rand_ca_basepath() {
  let i="($RANDOM % $ca_count) + 1"
  echo $(ca_i_basepath ${i})
}

build() {
  source $(dirname $0)/utils.sh
  tmp_dir=$(mktemp -d)
  temp_build ${tmp_dir} $@
}

case $1 in
  --help)
    echo "usage: $(basename $0) [setup|alpha|unauthorized]"
    ;;
  setup)
	build captplanet identity
	echo "setting up captplanet"
	"$captplanet" setup --overwrite
    echo "clearing whitelist"
    echo > ${ca_whitelist}
    echo -n "generating alpha certificate authorities.."
    for i in $(seq 1 ${ca_count}); do
      echo -n "$i.."
      _basepath=$(ca_i_basepath ${i})
      ${identity} ca new --ca.overwrite \
                      --ca.cert-path ${_basepath}.cert \
                      --ca.key-path ${_basepath}.key
      cat ${_basepath}.cert >> ${ca_whitelist}
    done
    echo "done"
    echo -n "generating alpha identities"
    for dir in ${basepath}/{f*,sat*,up*}; do
      echo -n "."
      _ca_basepath=$(rand_ca_basepath)
      _ca_cert=${dir}/ca-alpha.cert
      _ca_key=${dir}/ca-alpha.key
      ${identity} ca new --ca.overwrite \
                      --ca.cert-path ${_ca_cert} \
                      --ca.key-path ${_ca_key} \
                      --ca.parent-cert-path ${_ca_basepath}.cert \
                      --ca.parent-key-path ${_ca_basepath}.key
      ${identity} id new --identity.overwrite \
                      --identity.cert-path ${dir}/identity-alpha.cert \
                      --identity.key-path ${dir}/identity-alpha.key \
                      --ca.cert-path ${_ca_cert} \
                      --ca.key-path ${_ca_key}
    done
    echo "done"
    echo "writing alpha config"
    cat ${basepath}/config.yaml | \
		sed "s,peer-ca-whitelist-path: \"\",peer-ca-whitelist-path: $ca_whitelist,g" | \
		sed -E 's,cert-path: (.+)\.cert,cert-path: \1-alpha.cert,g' | \
		sed -E 's,key-path: (.+)\.key,key-path: \1-alpha.key,g' \
		> ${alpha_config}
    echo "writing unauthorized config"
    cat ${basepath}/config.yaml | sed -E "s,peer-ca-whitelist-path: \"\",peer-ca-whitelist-path: $ca_whitelist,g" > "$unauthorized_config"
    ;;
  alpha)
    build captplanet
    ${captplanet} run --config ${alpha_config}
    ;;
  unauthorized)
    build captplanet
    ${captplanet} run --config ${unauthorized_config}
    ;;
  run)
    ${captplanet} run
    ;;
  *)
    $0 --help
    ;;
esac

rm -rf ${tmp_dir}
