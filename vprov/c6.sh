#!/usr/bin/env bash

# CentOS 6

go_base_url="$1"
go_ver="$2"
go_install=0

# need to add a repo that has a later version of git - centos6 v1.7.1
# makes go mod's use of git to get versions fail...
cat > /etc/yum.repos.d/wandisco-git.repo <<"EORF"
[wandisco-git]
name=Wandisco GIT Repository
baseurl=http://opensource.wandisco.com/centos/6/git/$basearch/
enabled=1
gpgcheck=1
gpgkey=http://opensource.wandisco.com/RPM-GPG-KEY-WANdisco
EORF
rpm --import http://opensource.wandisco.com/RPM-GPG-KEY-WANdisco

echo "Installing needed packages (e.g. git, go, etc.)"
yum -q -e 0 makecache fast
yum -q install -y git rpm-build redhat-rpm-config gcc libpcap-devel

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

mkdir -p ~vagrant/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS} && chown -R vagrant:vagrant ~vagrant/rpmbuild
echo '%_topdir %(echo $HOME)/rpmbuild' > ~vagrant/.rpmmacros
chown vagrant:vagrant ~vagrant/godev ~vagrant/.rpmmacros ~vagrant/.bashrc

echo "provisioning complete"
# DONE
