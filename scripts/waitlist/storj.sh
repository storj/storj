#!/usr/bin/env bash
source $(dirname $0)/../utils.sh

comment() {
	cat << EOF
-----BEGIN COMMENT-----
Label: $1
Description: $2
-----END COMMENT-----
EOF
}

case $1 in
	--help)
		echo "usage: $0 new"
	;;
	new)
		shift
		check_help $1 	"usage: storj.sh new <label> <output dir> [<whitelist path>]"
		temp_build identity
		label=$1
		out_dir=$2
		whitelist=$3
		cert_path=${out_dir}/${label}.cert
		key_path=${out_dir}/${label}.key

		ensure_dir ${out_dir}
		no_overwrite ${cert_path}
		no_overwrite ${key_path}
		${identity} ca new \
			--ca.cert-path ${cert_path} \
			--ca.key-path ${key_path}

		echo "wrote:"
		log_list ${cert_path} ${key_path}

		if [ $# -gt 2 ]; then
			comment ${label} >> ${whitelist}
			cat ${cert_path} >> ${whitelist}
			echo "appended to whitelist at $whitelist"
		fi
	;;
	*)
		$0 --help
	;;
esac