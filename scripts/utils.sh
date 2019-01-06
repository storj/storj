#!/usr/bin/env bash

dots() {
	echo -n "."
	sleep 1
	dots
}

dots_on() {
	dots &
	dots_pid=$!
}

dots_off() {
	disown $dots_pid
	kill "$dots_pid"
}

build() {
	local tmp_dir=$1
	shift
	echo "building temp binaries:"
	for cmd in $@; do
		echo -n "	building $cmd..."
		dots_on
		local path=${tmp_dir}/${cmd}
		declare -g ${cmd}=${path}
		go build -o ${path} storj.io/storj/cmd/${cmd}
		dots_off
		echo "done"
	done
	echo "	binaries built in $tmp_dir"
}

temp_build() {
    declare -g tmp_build_dir=$(mktemp -d)
	build ${tmp_build_dir} $@
}

declare_cmds() {
	echo "using installed binaries:"
	for cmd in $@; do
        echo "	 - ${cmd}"
		declare -g ${cmd}=${cmd}
	done
}

check_help() {
	if [ $1 == "--help" ] || [ $1 == "-h" ]; then
		echo $2
		exit 0
	fi
}

ensure_dir() {
	if [ ! -d $1 ]; then
		mkdir $1
	fi
}

no_overwrite() {
	if [ -e $1 ]; then
	echo "Error: $1 already exists; refusing to overwrite"
		exit 10
	fi
}

log_list() {
	for f in $@; do
		echo ${f}
	done
}
