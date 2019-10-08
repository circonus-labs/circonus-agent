#!/usr/bin/env bash

set -e

distros="c7 c6 u18 u16 fb11"
[[ -n "$1" ]] && distros="${@:1}"

src_dir="~/godev/src/github.com/circonus-labs/circonus-agent/package"

for dist in $distros; do
    cmd="cd $src_dir && ./build.sh"
    [[ $dist == "fb11" ]] && { tmp="/usr/local/bin/bash -l -c 'cd $src_dir && cp build.sh ~/. && ~/build.sh'"; cmd=$tmp; }
    vagrant up $dist
    vagrant ssh $dist --command="${cmd}"
    vagrant halt $dist
    echo
    echo "Waiting 5 seconds [CTRL-C] to abort builds..."
    echo
    sleep 5
done

if [[ -n "$CA_BUILDS_DEST" ]]; then
    [[ ! -d $CA_BUILDS_DEST ]] && { echo "invalid build destination (${CA_BUILDS_DEST}), directory does not exist"; exit 1; }
    set +e # these can fail, e.g. building a single package
    echo "copying packages to cosi-examples/server for provisioning"
    for ext in rpm deb tgz; do
        cp -v package/publish/circonus-agent-*.$ext ${CA_BUILDS_DEST}/.
    done
fi