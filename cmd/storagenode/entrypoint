#!/bin/sh
set -euo pipefail

if [[ ! -f "config/config.yaml" ]]; then
	./storagenode setup --config-dir config --identity-dir identity
fi

RUN_PARAMS="${RUN_PARAMS:-} --config-dir config"
RUN_PARAMS="${RUN_PARAMS:-} --identity-dir identity"

RUN_PARAMS="${RUN_PARAMS:-} --kademlia.bootstrap-addr=bootstrap.storj.io:8888"
RUN_PARAMS="${RUN_PARAMS:-} --metrics.app-suffix=-alpha"
RUN_PARAMS="${RUN_PARAMS:-} --metrics.interval=30m"
RUN_PARAMS="${RUN_PARAMS:-} --server.use-peer-ca-whitelist=true"
RUN_PARAMS="${RUN_PARAMS:-} --kademlia.external-address=${ADDRESS}"
RUN_PARAMS="${RUN_PARAMS:-} --kademlia.operator.email=${EMAIL}"
RUN_PARAMS="${RUN_PARAMS:-} --kademlia.operator.wallet=${WALLET}"
RUN_PARAMS="${RUN_PARAMS:-} --storage.allocated-bandwidth=${BANDWIDTH}"
RUN_PARAMS="${RUN_PARAMS:-} --storage.allocated-disk-space=${STORAGE}"
RUN_PARAMS="${RUN_PARAMS:-} --storage.whitelisted-satellite-i-ds=12EayRS2V1kEsWESU9QMRseFhdxYxKicsiFmxrsLZHeLUtdps3S,1Vrf9xmmHw6KaVFMcfR2YPt8YpVVoQZGTUJyjYc6CajeYrAqrB,118UWpMCHzs6CvSgWd9BfFVjw5K9pZbJjkfZJexMtSkmKxvvAW"

exec ./storagenode run $RUN_PARAMS "$@"
