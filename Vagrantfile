# -*- mode: ruby -*-
# vi: set ft=ruby :
# rubocop:disable Metrics/BlockLength
#
# defines VMs for developing/testing Circonus Agent
#

Vagrant.configure('2') do |config|
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
                go_ver="1.11"
                go_tgz="go${go_ver}.linux-amd64.tar.gz"
                [[ -f /vagrant/${go_tgz} ]] || {
                    curl -sSL "https://storage.googleapis.com/golang/${go_tgz}" -o /home/vagrant/$go_tgz
                    [[ $? -eq 0 ]] || { echo "Unable to download go tgz"; exit 1; }
                }
                tar -C /usr/local -xzf /home/vagrant/$go_tgz
                [[ $? -eq 0 ]] || { echo "Error unarchiving $go_tgz"; exit 1; }
            fi
            [[ -f /etc/profile.d/go.sh ]] || echo 'export PATH="$PATH:/usr/local/go/bin"' > /etc/profile.d/go.sh
            [[ $(grep -c GOPATH /home/vagrant/.bashrc) -eq 0 ]] && echo 'export GOPATH="${HOME}/godev"' >> /home/vagrant/.bashrc
            mkdir -p ~/rpmbuild/{BUILD,RPMS,SOURCES,SPECS,SRPMS}
            echo '%_topdir %(echo $HOME)/rpmbuild' > ~/.rpmmacros
            sudo chown vagrant:vagrant ~vagrant/godev
        SHELL
    end
end
