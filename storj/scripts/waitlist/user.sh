#!/usr/bin/env bash
source $(dirname $0)/../utils.sh

new_ca() {
	${identity} ca new \
		--ca.cert-path ${ca_cert_path} \
		--ca.key-path ${ca_key_path} \
		--ca.parent-cert-path ${parent_cert_path} \
		--ca.parent-key-path ${parent_key_path}
}

case $1 in
	--help)
		echo "usage: user.sh new|batch"
	;;
	new)
		shift
		check_help $1 "usage: identity.sh new <parent dir> <parent label> <label> <output dir>"
		temp_build identity
		parent_cert_path=${1}/${2}.cert
		parent_key_path=${1}/${2}.key
		ca_cert_path=${4}/${3}.cert
		ca_key_path=${4}/${3}.key

		ensure_dir $4
		no_overwrite ${ca_cert_path}
		no_overwrite ${ca_key_path}
		new_ca

		echo "wrote:"
		log_list ${ca_cert_path} ${ca_key_path}
		echo "certificate signed by cert:"
		log_list ${parent_cert_path} ${parent_key_path}
	;;
	batch)
		shift
		check_help $1 "usage: user.sh batch <labels file> <parent dir> <parent label> <output dir>"
		temp_build identity
		labels=$(cat $1)

		for label in ${labels}; do
			parent_cert_path=${2}/${3}.cert
			parent_key_path=${2}/${3}.key
			ca_cert_path=${4}/${label}.cert
			ca_key_path=${4}/${label}.key

			ensure_dir $4
			no_overwrite ${ca_cert_path}
			no_overwrite ${ca_key_path}
			new_ca

			log_list ${ca_cert_path} ${ca_key_path}
		done

		echo "certificates signed by cert:"
		log_list ${parent_cert_path} ${parent_key_path}
	;;
esac
