# -*- mode: ruby -*-
# vi: set ft=ruby :
# rubocop:disable Metrics/BlockLength
#
# defines VMs for developing/testing Circonus Agent
#

go_ver = `go version`.chomp.split(' ')[2]
go_url_base = 'https://dl.google.com/go'
agent_src_path = '/home/vagrant/godev/src/github.com/circonus-labs/circonus-agent'

Vagrant.configure('2') do |config|
    #
    # el7 builder
    #
    config.vm.define 'c7', autostart: false do |c7|
        c7.vm.box = 'maier/centos-7.4.1708-x86_64'
        c7.vm.provider 'virtualbox' do |vb|
            vb.name = 'c7_circonus-agent_build'
            vb.cpus = 2
        end
        c7.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        c7.vm.network 'private_network', ip: '192.168.100.240'
        c7.vm.provision 'shell', path: 'vprov/c7.sh', args: [go_url_base, go_ver]
    end
    #
    # el6 builder
    #
    config.vm.define 'c6', autostart: false do |c6|
        c6.vm.box = 'maier/centos-6.8-x86_64'
        c6.vm.provider 'virtualbox' do |vb|
            vb.name = 'c6_circonus-agent_build'
            vb.cpus = 2
        end
        c6.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        c6.vm.network 'private_network', ip: '192.168.100.241'
        c6.vm.provision 'shell', path: 'vprov/c6.sh', args: [go_url_base, go_ver]
    end
    #
    # ubuntu18 builder
    #
    config.vm.define 'u18', autostart: false do |u18|
        #u18.vm.box = 'maier/ubuntu-18.04-x86_64'
        u18.vm.box = 'ubuntu/bionic64'
        u18.vm.provider 'virtualbox' do |vb|
            vb.name = 'u18_circonus-agent_build'
            vb.cpus = 2
            vb.customize ['modifyvm', :id, '--uartmode1', 'disconnected']
        end
        u18.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        u18.vm.network 'private_network', ip: '192.168.100.242'
        u18.vm.provision 'shell', path: 'vprov/u18.sh', args: [go_url_base, go_ver]
    end
    #
    # ubuntu16 builder
    #
    config.vm.define 'u16', autostart: false do |u16|
        u16.vm.box = 'maier/ubuntu-16.04-x86_64'
        u16.vm.provider 'virtualbox' do |vb|
            vb.name = 'u16_circonus-agent_build'
            vb.cpus = 2
            vb.customize ['modifyvm', :id, '--uartmode1', 'disconnected']
        end
        u16.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        u16.vm.network 'private_network', ip: '192.168.100.243'
        u16.vm.provision 'shell', path: 'vprov/u16.sh', args: [go_url_base, go_ver]
    end
    # #
    # # ubuntu14 builder EOL
    # #
    # config.vm.define 'u14', autostart: false do |u14|
    #     u14.vm.box = 'maier/ubuntu-14.04-x86_64'
    #     u14.vm.provider 'virtualbox' do |vb|
    #         vb.name = 'u14_circonus-agent_build'
    #         vb.cpus = 2
    #         vb.customize ['modifyvm', :id, '--uartmode1', 'disconnected']
    #     end
    #     u14.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
    #     u14.vm.network 'private_network', ip: '192.168.100.244'
    #     u14.vm.provision 'shell', path: 'vprov/u14.sh', args: [go_url_base, go_ver]
    # end

    # #
    # # OmniOS r15 EOL
    # #
    # config.vm.define 'o15', autostart: false do |o15|
    #     o15.vm.box = 'maier/omnios-r151014-x86_64'
    #     o15.vm.provider 'virtualbox' do |vb|
    #         vb.name = 'o15_circonus-agent_build'
    #         vb.cpus = 2
    #     end
    #     o15.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
    #     # o15.vm.network 'private_network', ip: '192.168.100.245'
    #     o15.vm.provision 'shell', inline: <<-SHELL
    #         echo "Installing needed packages (e.g. git, go, etc.)"
    #         pkg set-publisher -g http://updates.circonus.net/omnios/r151014/ circonus
    #         pkg install -q developer/gcc48
    #         [[ $(grep -c "PATH" /root/.bashrc) -eq 0  ]] && {
    #             echo '[[ -f ~/.bashrc ]] && source ~/.bashrc' >> /root/.profile
    #             echo 'export PATH="$PATH:$(ls -d /opt/gcc*)/bin"' >> /root/.bashrc
    #         }
    #     SHELL
    # end

    #
    # FreeBSD 11 builder
    #
    config.vm.define 'fb11', autostart: false do |fb11|
        fb11.vm.guest = :freebsd
        fb11.vm.box = 'freebsd/FreeBSD-11.2-RELEASE'
        # doesn't work correctly, consistently... fb11.vm.synced_folder '.', agent_src_path, id: 'vagrant-root'
        fb11.vm.synced_folder '.', '/vagrant', id: 'vagrant-root', disabled: true
        fb11.vm.synced_folder '.', agent_src_path, type: 'nfs'
        # mac not set in base box, just needs to be set to something to avoid vagrant errors
        fb11.vm.base_mac = ''
        fb11.ssh.shell = 'sh'
        fb11.vm.provider 'virtualbox' do |vb|
            vb.name = 'fb11_circonus-agent_build'
            vb.customize ['modifyvm', :id, '--memory', '2048']
            vb.customize ['modifyvm', :id, '--cpus', '2']
            vb.customize ['modifyvm', :id, '--hwvirtex', 'on']
            vb.customize ['modifyvm', :id, '--audio', 'none']
            vb.customize ['modifyvm', :id, '--nictype1', 'virtio']
            vb.customize ['modifyvm', :id, '--nictype2', 'virtio']
        end
        fb11.vm.network 'private_network', ip: '192.168.100.246'
        fb11.vm.provision 'bootstrap', type: 'shell', inline: <<-SHELL
            echo "Installing needed packages (e.g. git, gcc, etc.)"
            pkg install -y -q git gcc gmake bash logrotate curl
            chsh -s /usr/local/bin/bash vagrant
        SHELL
        fb11.vm.provision 'shell', path: 'vprov/fb11.sh', args: [go_url_base, go_ver]
        fb11.trigger.after(:halt) do |trigger|
            trigger.info = "Purging NFSD exports"
            trigger.ruby do |_env, _machine|
                etc_exports = Pathname.new("/etc/exports")
                if etc_exports.exist? && etc_exports.size > 0
                    system(%q{sudo cp /dev/null /etc/exports && sudo nfsd restart})
                end
            end
        end
    end
end
