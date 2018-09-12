#!/usr/bin/env bash

# Ubuntu 16

go_base_url="$1"
go_ver="$2"

echo "Installing needed packages (e.g. git, go, etc.)"
apt-get update
apt-get --assume-yes install git ruby ruby-dev rubygems build-essential libpcap-dev

echo "Installing FPM gem"
gem install --no-ri --no-rdoc fpm

if [[ ! -x /usr/local/go/bin/go ]]; then
    go_tgz="go${go_ver}.linux-amd64.tar.gz"
    [[ -f /vagrant/${go_tgz} ]] || {
        go_url="${go_base_url}/${go_tgz}"
        echo "Downloading ${go_url}"
        curl -sSL "$go_url" -o ~vagrant/$go_tgz
        [[ $? -eq 0 ]] || { echo "Unable to download go tgz"; exit 1; }
    }
    tar -C /usr/local -xf ~vagrant/$go_tgz
    [[ $? -eq 0 ]] || { echo "Error unarchiving $go_tgz"; exit 1; }
fi

[[ -f /etc/profile.d/go.sh ]] || echo 'export PATH="$PATH:/usr/local/go/bin"' > /etc/profile.d/go.sh
[[ $(grep -c GOPATH ~vagrant/.bashrc) -eq 0 ]] && echo 'export GOPATH="${HOME}/godev"' >> ~vagrant/.bashrc

chown vagrant:vagrant ~vagrant/godev ~vagrant/.bashrc
