#!/usr/bin/env bash
set -ueo pipefail
set +x

. $(dirname $0)/utils.sh

temp_build inspector
TMPDIR=$(mktemp -d -t tmp.XXXXXXXXXX)
trap "rm -rf ${tmp_build_dir} ${TMPDIR}" ERR INT EXIT

map_dot=${TMPDIR}/map.dot
#$inspector map --identity-path ~/.local/share/storj/identity/inspector bootstrap.storj.io:8888 > ${map_dot}
#$inspector map --identity-path ~/.local/share/storj/identity/inspector 0.tcp.ngrok.io:16151 > ${map_dot} 2> $(pwd)/map-err.log
$inspector map-network --identity-path ~/.local/share/storj/identity/inspector 0.tcp.ngrok.io:16151 > $(pwd)/map.dot 2> $(pwd)/map-err.log

#dot -T svg ${map_dot} > $(pwd)/map-dot.svg
#neato -T svg ${map_dot} > $(pwd)/map-neato.svg
circo -T svg ${map_dot} > $(pwd)/map-circo.svg
fdp -T svg ${map_dot} > $(pwd)/map-fdp.svg
sfdp -T svg ${map_dot} > $(pwd)/map-sfdp.svg
#patchwork -T svg ${map_dot} > $(pwd)/map-patchwork.svg
#osage -T svg ${map_dot} > $(pwd)/map-osage.svg
