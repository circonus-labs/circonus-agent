# -*- mode: ruby -*-
# vi: set ft=ruby :
# rubocop:disable Metrics/BlockLength
#
# defines VMs for developing/testing Circonus Agent
#

go_ver = '1.11'
go_url_base = 'https://dl.google.com/go'

Vagrant.configure('2') do |config|
    #
    # el7 builder
    #
    config.vm.define 'c7', autostart: false do |c7|
        c7.vm.box = 'maier/centos-7.3.1611-x86_64'
        c7.vm.provider 'virtualbox' do |vb|
            vb.name = 'c7_circonus-agent'
            vb.cpus = 2
        end
        c7.vm.synced_folder '.',
                            '/home/vagrant/godev/src/github.com/circonus-labs/circonus-agent',
                            owner: 'vagrant',
                            group: 'vagrant'
        c7.vm.network 'private_network', ip: '192.168.100.240'
        c7.vm.provision 'shell', inline: <<-SHELL
            yum -q -e 0 makecache fast
            echo "Installing needed packages (e.g. git, go, etc.)"
            yum -q install -y git rpm-build redhat-rpm-config gcc libpcap-devel
            if [[ ! -x /usr/local/go/bin/go ]]; then
                go_tgz="go#{go_ver}.linux-amd64.tar.gz"
                [[ -f /vagrant/${go_tgz} ]] || {
                    go_url="#{go_url_base}/${go_tgz}"
                    echo "Downloading ${go_url}"
                    curl -sSL "$go_url" -o /home/vagrant/$go_tgz
                    [[ $? -eq 0 ]] || { echo "Unable to download go tgz"; exit 1; }
                }
                tar -C /usr/local -xzf /home/vagrant/$go_tgz
                [[ $? -eq 0 ]] || { echo "Error unarchiving $go_tgz"; exit 1; }
            fi
            [[ -f /etc/profile.d/go.sh ]] || echo 'export PATH="$PATH:/usr/local/go/bin"' > /etc/profile.d/go.sh
            [[ $(grep -c GOPATH /home/vagrant/.bashrc) -eq 0 ]] && echo 'export GOPATH="${HOME}/godev"' >> /home/vagrant/.bashrc
            mkdir -p ~vagrant/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS} && chown -R vagrant:vagrant ~vagrant/rpmbuild
            echo '%_topdir %(echo $HOME)/rpmbuild' > ~vagrant/.rpmmacros
            chown vagrant:vagrant ~vagrant/godev ~vagrant/.rpmmacros
        SHELL
    end
    #
    # el6 builder
    #
    config.vm.define 'c6', autostart: false do |c6|
        c6.vm.box = 'maier/centos-6.8-x86_64'
        c6.vm.provider 'virtualbox' do |vb|
            vb.name = 'c6_circonus-agent'
            vb.cpus = 2
        end
        c6.vm.synced_folder '.',
                            '/home/vagrant/godev/src/github.com/circonus-labs/circonus-agent',
                            owner: 'vagrant',
                            group: 'vagrant'
        c6.vm.network 'private_network', ip: '192.168.100.241'
        c6.vm.provision 'shell', inline: <<-SHELL
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
            yum -q -e 0 makecache fast
            echo "Installing needed packages (e.g. git, go, etc.)"
            yum -q install -y git rpm-build redhat-rpm-config gcc libpcap-devel
            if [[ ! -x /usr/local/go/bin/go ]]; then
                go_tgz="go#{go_ver}.linux-amd64.tar.gz"
                [[ -f /vagrant/${go_tgz} ]] || {
                    go_url="#{go_url_base}/${go_tgz}"
                    echo "Downloading ${go_url}"
                    curl -sSL "$go_url" -o /home/vagrant/$go_tgz
                    [[ $? -eq 0 ]] || { echo "Unable to download go tgz"; exit 1; }
                }
                tar -C /usr/local -xf /home/vagrant/$go_tgz
                [[ $? -eq 0 ]] || { echo "Error unarchiving $go_tgz"; exit 1; }
            fi
            [[ -f /etc/profile.d/go.sh ]] || echo 'export PATH="$PATH:/usr/local/go/bin"' > /etc/profile.d/go.sh
            [[ $(grep -c GOPATH /home/vagrant/.bashrc) -eq 0 ]] && echo 'export GOPATH="${HOME}/godev"' >> /home/vagrant/.bashrc
            mkdir -p ~vagrant/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS} && chown -R vagrant:vagrant ~vagrant/rpmbuild
            echo '%_topdir %(echo $HOME)/rpmbuild' > ~vagrant/.rpmmacros
            chown vagrant:vagrant ~vagrant/godev ~vagrant/.rpmmacros
        SHELL
    end
end
