#!/usr/bin/env bash

set -e

distros="el8 el7 u22 u20"
[[ -n "$1" ]] && distros="${@:1}"

src_dir="~/godev/src/github.com/circonus-labs/circonus-agent/package"

for dist in $distros; do
    cmd="cd $src_dir && ./build.sh"
    [[ $dist == "fb11" ]] && { tmp="/usr/local/bin/bash -l -c 'cd $src_dir && cp build.sh /home/vagrant/. && /home/vagrant/build.sh'"; cmd=$tmp; }
    [[ $dist == "fb12" ]] && { tmp="/usr/local/bin/bash -l -c 'cd $src_dir && cp build.sh /home/vagrant/. && /home/vagrant/build.sh'"; cmd=$tmp; }
    vagrant up $dist
    vagrant ssh $dist --command="${cmd}"
    vagrant halt $dist
    # ubuntu ... tty output
    [[ -f console.log ]] && rm console.log
    echo
    echo "Waiting 5 seconds [CTRL-C] to abort builds..."
    echo
    sleep 5
done

sig_file="package_checksums.txt"
agent_version=$(git describe --abbrev=0 --tags)
pub_dir="package/publish/${agent_version}"
pushd $pub_dir
[[ -f $sig_file ]] && rm $sig_file
echo "Creating $sig_file"
for f in circonus-agent*; do
    echo "  processing $f"
    shasum -a 256 $f >> $sig_file
done


#if [[ -n "$CA_BUILDS_DEST" ]]; then
#   [[ ! -d $CA_BUILDS_DEST ]] && { echo "invalid build destination (${CA_BUILDS_DEST}), directory does not exist"; exit 1; }
#   set +e # these can fail, e.g. building a single package
#   echo "copying packages to cosi-examples/server for provisioning"
#   for ext in rpm deb tgz; do
#       cp -v package/publish/circonus-agent-*.$ext ${CA_BUILDS_DEST}
#   done
#fi
