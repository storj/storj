#!/usr/bin/env bash

new_error() {
  file=$0
  err_msg=$1
  line_no=$2

    echo -e "ERROR: ${file}: line ${line_no}: ${err_msg}"
    exit 1
}

require_empty() {
  line_no=$2

  if [[ -z $(sed -e 's/^[[:space:]]*//') ]]; then
    new_error "expected \"$1\" to be an empty string" $line_no
  fi
}

require_equal() {
  a=$1
  b=$2
  line_no=$3

  if [[ "$a" != "$b" ]]; then
    new_error "expected equal:\n$(diff <(echo $a) <(echo $b))" $line_no
  fi
}

require_lines() {
  line_no=$3
  string=$2
  line_count=$(echo "$string" | wc -l)
  if [[ "$line_count" -lt "$1" ]]; then
    new_error "expected number of lines ${line_count} to be ${1}:\n$2" $line_no
  fi
}

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

build_cleanup() {
    dots_off
    echo
    echo "BUILD ERROR:"
    echo "$build_out"
}
build() {
    trap "build_cleanup" ERR
	local tmp_dir=$1
	shift
	echo "building temp binaries:"
	for cmd in $@; do
		echo -n "	building $cmd..."
		dots_on
		local path=${tmp_dir}/${cmd}
		declare -g $(echo $cmd | sed s,-,_,g)=${path}
		build_out=$(go build -o ${path} storj.io/storj/cmd/${cmd} 2>&1)
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
