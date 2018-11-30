#!/usr/bin/env bash

# CentOS 7

go_base_url="$1"
go_ver="$2"

echo "Installing needed packages (e.g. git, go, etc.)"
yum -q -e 0 makecache fast
yum -q install -y git rpm-build redhat-rpm-config gcc libpcap-devel

if [[ ! -x /usr/local/go/bin/go ]]; then
    go_tgz="go${go_ver}.linux-amd64.tar.gz"
    [[ -f /vagrant/${go_tgz} ]] || {
        go_url="${go_base_url}/${go_tgz}"
        echo "Downloading ${go_url}"
        curl -sSL "$go_url" -o ~vagrant/$go_tgz
        [[ $? -eq 0 ]] || { echo "Unable to download go tgz"; exit 1; }
    }
    echo "Installing go ${go_ver} in /usr/local"
    tar -C /usr/local -xf ~vagrant/$go_tgz
    [[ $? -eq 0 ]] || { echo "Error unarchiving $go_tgz"; exit 1; }
fi

[[ -f /etc/profile.d/go.sh ]] || echo 'export PATH="$PATH:/usr/local/go/bin"' > /etc/profile.d/go.sh
[[ $(grep -c GOPATH ~vagrant/.bashrc) -eq 0 ]] && echo 'export GOPATH="${HOME}/godev"' >> ~vagrant/.bashrc

mkdir -p ~vagrant/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS} && chown -R vagrant:vagrant ~vagrant/rpmbuild
echo '%_topdir %(echo $HOME)/rpmbuild' > ~vagrant/.rpmmacros
chown vagrant:vagrant ~vagrant/godev ~vagrant/.rpmmacros ~vagrant/.bashrc
