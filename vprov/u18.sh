#!/usr/bin/env bash

# Ubuntu 18

go_base_url="$1"
go_ver="$2"
go_install=0

echo "Installing needed packages (e.g. git, go, etc.)"
DEBIAN_FRONTEND=noninteractive apt-get -yq update
DEBIAN_FRONTEND=noninteractive apt-get -yq install git ruby ruby-dev rubygems build-essential libpcap-dev

echo "Installing FPM gem"
gem install --no-ri --no-rdoc fpm

if [[ ! -x /usr/local/go/bin/go ]]; then
    go_install=1
    echo "want $go_ver, not found - INSTALLING"
else
    gov=`/usr/local/go/bin/go version | cut -d ' ' -f 3`
    if [[ "$gov" == "$go_ver" ]]; then
        echo "want $go_ver, found $gov - OK"
    else
        go_install=1
        echo "want $go_ver, found $gov - UPGRADING"
    fi
fi

if [[ $go_install -eq 1 ]]; then
    go_tgz="${go_ver}.linux-amd64.tar.gz"
    [[ -f /vagrant/${go_tgz} ]] || {
        go_url="${go_base_url}/${go_tgz}"
        echo "downloading ${go_url}"
        curl -sSL "$go_url" -o /home/vagrant/$go_tgz
        [[ $? -eq 0 ]] || { echo "Unable to download go tgz"; exit 1; }
    }
    echo "installing ${go_tgz} in /usr/local"
    $SUDO tar -C /usr/local -xf /home/vagrant/$go_tgz
    [[ $? -eq 0 ]] || { echo "Error unarchiving $go_tgz"; exit 1; }
fi

[[ -f /etc/profile.d/go.sh ]] || echo 'export PATH="$PATH:/usr/local/go/bin"' > /etc/profile.d/go.sh
[[ $(grep -c GOPATH ~vagrant/.bashrc) -eq 0 ]] && echo 'export GOPATH="${HOME}/godev"' >> ~vagrant/.bashrc
chown vagrant:vagrant ~vagrant/godev ~vagrant/.bashrc

echo "provisioning complete"

echo "Checking for (and installing) kernel upgrade..."
DEBIAN_FRONTEND=noninteractive apt -yq install linux-generic-hwe-18.04
[[ $? -eq 0 ]] && echo "New kernel may have been installed, use: vagrant reload u18"
# DONE
