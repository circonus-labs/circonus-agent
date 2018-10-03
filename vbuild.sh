#!/usr/bin/env bash

set -e

distros="c7 c6 u18 u16 u14 fb11"
[[ -n "$1" ]] && distros="$1"

for dist in $distros; do
    vagrant up $dist && \
    vagrant ssh $dist --command="cd ~/godev/src/github.com/circonus-labs/circonus-agent/package && ./build.sh" && \
    vagrant halt $dist
    echo "Waiting 5 seconds [CTRL-C] to abort builds..."
    sleep 5
done
