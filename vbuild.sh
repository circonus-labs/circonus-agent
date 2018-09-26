#!/usr/bin/env bash

set -e

for dist in c7 c6 u16 u14 fb11; do
    vagrant up $dist && \
    vagrant ssh $dist --command="cd ~/godev/src/github.com/circonus-labs/circonus-agent/package && ./build.sh" && \
    vagrant halt $dist
    echo "Waiting 5 seconds [CTRL-C] to abort builds..."
    sleep 5
done
