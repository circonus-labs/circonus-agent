#!/usr/bin/env bash

set -e

distros="c7 c6 u18 u16 u14 fb11"
[[ -n "$1" ]] && distros="$1"

for dist in $distros; do
    cmd="cd ~/godev/src/github.com/circonus-labs/circonus-agent/package && ./build.sh"
    [[ $dist == "fb11" ]] && { tmp="/usr/local/bin/bash -l -c '${cmd}'"; cmd=$tmp; }
    vagrant up $dist && \
    vagrant ssh $dist --command="${cmd}" && \
    vagrant halt $dist
    echo
    echo "Waiting 5 seconds [CTRL-C] to abort builds..."
    echo
    sleep 5
done
echo "copying packages to cosi-examples/server for provisioning"
cp -v package/publish/circonus-agent-*.{rpm,deb,tgz} ../cosi-examples/server/provision/roles/server/files/packages/.
