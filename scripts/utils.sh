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

temp_build() {
	local tmp_dir=$1
	shift
	echo "building binaries:"
	for cmd in $@; do
		echo -n "  buliding $cmd..."
		dots_on
		local path=${tmp_dir}/${cmd}
		declare -g ${cmd}=${path}
		go build -o ${path} storj.io/storj/cmd/${cmd}
		#				sleep .5
		dots_off
		echo "done"
	done
	echo "binaries built in $tmp_dir"
}
