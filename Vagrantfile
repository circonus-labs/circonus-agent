# -*- mode: ruby -*-
# vi: set ft=ruby :
# rubocop:disable Metrics/BlockLength
#
# defines VMs for developing/testing Circonus Agent
#

go_ver = `go version`.chomp.split(' ')[2]
go_url_base = 'https://dl.google.com/go'
agent_src_path = '/home/vagrant/godev/src/github.com/circonus-labs/circonus-agent'

#
# NOTE: all VMs have the SAME network IP - they should never run simultaneously (intentionally)
#

Vagrant.configure('2') do |config|
    #
    # el8 builder
    #
    config.vm.define 'el8', autostart: false do |el8|
        el8.vm.box = 'centos/8'
        el8.vm.provider 'virtualbox' do |vb|
            vb.name = 'el8_circonus-agent_build'
            vb.memory = '2048'
            vb.cpus = 2
        end
        el8.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        el8.vm.network 'private_network', ip: '192.168.100.200'
        el8.vm.provision 'shell', path: 'vprov/el8.sh', args: [go_url_base, go_ver]
        el8.vbguest.auto_update = false
    end
    #
    # el7 builder
    #
    config.vm.define 'el7', autostart: false do |el7|
        # el7.vm.box = 'maier/centos-7.4.1708-x86_64'
        el7.vm.box = 'centos/7'
        el7.vm.provider 'virtualbox' do |vb|
            vb.name = 'el7_circonus-agent_build'
            vb.memory = '2048'
            vb.cpus = 2
        end
        el7.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        el7.vm.network 'private_network', ip: '192.168.100.200'
        el7.vm.provision 'shell', path: 'vprov/el7.sh', args: [go_url_base, go_ver]
        el7.vbguest.auto_update = true
    end
    #
    # el6 builder
    #
    config.vm.define 'el6', autostart: false do |el6|
        # el6.vm.box = 'maier/centos-6.8-x86_64'
        el6.vm.box = 'centos/6'
        el6.vm.provider 'virtualbox' do |vb|
            vb.name = 'el6_circonus-agent_build'
            vb.memory = '2048'
            vb.cpus = 2
        end
        el6.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        el6.vm.network 'private_network', ip: '192.168.100.200'
        el6.vm.provision 'shell', path: 'vprov/el6.sh', args: [go_url_base, go_ver]
        el6.vbguest.auto_update = true
    end
    #
    # ubuntu20 builder
    #
    config.vm.define 'u20', autostart: false do |u20|
        u20.vm.box = 'ubuntu/focal64'
        u20.vm.provider 'virtualbox' do |vb|
            vb.name = 'u20_circonus-agent_build'
            vb.memory = '2048'
            vb.cpus = 2
            vb.customize [ "modifyvm", :id, "--uartmode1", "file", File.join(Dir.pwd, "./console.log")]
        end
        u20.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        u20.vm.network 'private_network', ip: '192.168.100.200'
        u20.vm.provision 'shell', path: 'vprov/u20.sh', args: [go_url_base, go_ver]
        u20.vbguest.auto_update = false
    end
    #
    # ubuntu18 builder
    #
    config.vm.define 'u18', autostart: false do |u18|
        u18.vm.box = 'ubuntu/bionic64'
        u18.vm.provider 'virtualbox' do |vb|
            vb.name = 'u18_circonus-agent_build'
            vb.memory = '2048'
            vb.cpus = 2
            vb.customize [ "modifyvm", :id, "--uartmode1", "file", File.join(Dir.pwd, "./console.log")]
        end
        u18.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        u18.vm.network 'private_network', ip: '192.168.100.200'
        u18.vm.provision 'shell', path: 'vprov/u18.sh', args: [go_url_base, go_ver]
        u18.vbguest.auto_update = true
    end
    #
    # ubuntu16 builder
    #
    config.vm.define 'u16', autostart: false do |u16|
        u16.vm.box = 'ubuntu/xenial64'
        u16.vm.provider 'virtualbox' do |vb|
            vb.name = 'u16_circonus-agent_build'
            vb.memory = '2048'
            vb.cpus = 2
            vb.customize [ "modifyvm", :id, "--uartmode1", "file", File.join(Dir.pwd, "./console.log")]
        end
        u16.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        u16.vm.network 'private_network', ip: '192.168.100.200'
        u16.vm.provision 'shell', path: 'vprov/u16.sh', args: [go_url_base, go_ver]
        u16.vbguest.auto_update = true
    end
    #
    # ubuntu14 builder
    #
    config.vm.define 'u14', autostart: false do |u14|
        u14.vm.box = 'ubuntu/trusty64'
        u14.vm.provider 'virtualbox' do |vb|
            vb.name = 'u14_circonus-agent_build'
            vb.memory = '2048'
            vb.cpus = 2
            vb.customize [ "modifyvm", :id, "--uartmode1", "file", File.join(Dir.pwd, "./console.log")]
        end
        u14.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant'
        u14.vm.network 'private_network', ip: '192.168.100.200'
        u14.vm.provision 'shell', path: 'vprov/u14.sh', args: [go_url_base, go_ver]
        u14.vbguest.auto_update = true
    end

    #
    # FreeBSD 12 builder
    #
    config.vm.define 'fb12', autostart: false do |fb12|
        fb12.vm.guest = :freebsd
        fb12.vm.box = 'freebsd/FreeBSD-12.1-STABLE'
        fb12.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant', type: "virtualbox"
        # mac not set in base box, just needs to be set to something to avoid vagrant errors
        fb12.vm.base_mac = ''
        fb12.ssh.shell = 'sh'
        fb12.vm.provider 'virtualbox' do |vb|
            vb.name = 'fb12_circonus-agent_build'
            vb.customize ['modifyvm', :id, '--memory', '2048']
            vb.customize ['modifyvm', :id, '--cpus', '2']
            vb.customize ['modifyvm', :id, '--hwvirtex', 'on']
            vb.customize ['modifyvm', :id, '--audio', 'none']
            vb.customize ['modifyvm', :id, '--nictype1', 'virtio']
            vb.customize ['modifyvm', :id, '--nictype2', 'virtio']
        end
        fb12.vm.network 'private_network', ip: '192.168.100.200'
        fb12.vbguest.auto_update = true
        fb12.vm.provision 'bootstrap', type: 'shell', inline: <<-SHELL
            echo "Installing needed packages (e.g. git, gcc, etc.)"
            pkg install -y -q git gcc gmake bash logrotate curl
            chsh -s /usr/local/bin/bash vagrant
        SHELL
        fb12.vm.provision 'shell', path: 'vprov/fb12.sh', args: [go_url_base, go_ver]
        # fb12.trigger.after(:halt) do |trigger|
        #     trigger.info = "Purging NFSD exports"
        #     trigger.ruby do |_env, _machine|
        #         etc_exports = Pathname.new("/etc/exports")
        #         if etc_exports.exist? && etc_exports.size > 0
        #             system(%q{sudo cp /dev/null /etc/exports && sudo nfsd restart})
        #         end
        #     end
        # end
    end
    #
    # FreeBSD 11 builder
    #
    config.vm.define 'fb11', autostart: false do |fb11|
        fb11.vm.guest = :freebsd
        fb11.vm.box = 'freebsd/FreeBSD-11.3-STABLE'
        fb11.vm.synced_folder '.', agent_src_path, owner: 'vagrant', group: 'vagrant', type: "virtualbox"
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
        fb11.vm.network 'private_network', ip: '192.168.100.200'
        fb11.vbguest.auto_update = true
        fb11.vm.provision 'bootstrap', type: 'shell', inline: <<-SHELL
            echo "Installing needed packages (e.g. git, gcc, etc.)"
            pkg install -y -q git gcc gmake bash logrotate curl
            chsh -s /usr/local/bin/bash vagrant
        SHELL
        fb11.vm.provision 'shell', path: 'vprov/fb11.sh', args: [go_url_base, go_ver]
        # fb11.trigger.after(:halt) do |trigger|
        #     trigger.info = "Purging NFSD exports"
        #     trigger.ruby do |_env, _machine|
        #         etc_exports = Pathname.new("/etc/exports")
        #         if etc_exports.exist? && etc_exports.size > 0
        #             system(%q{sudo cp /dev/null /etc/exports && sudo nfsd restart})
        #         end
        #     end
        # end
    end

    # #
    # # FreeBSD 11 builder
    # #
    # config.vm.define 'fb11', autostart: false do |fb11|
    #     fb11.vm.guest = :freebsd
    #     fb11.vm.box = 'freebsd/FreeBSD-11.2-RELEASE'
    #     # doesn't work correctly, consistently... fb11.vm.synced_folder '.', agent_src_path, id: 'vagrant-root'
    #     fb11.vm.synced_folder '.', '/vagrant', id: 'vagrant-root', disabled: true
    #     fb11.vm.synced_folder '.', agent_src_path, type: 'nfs'
    #     # mac not set in base box, just needs to be set to something to avoid vagrant errors
    #     fb11.vm.base_mac = ''
    #     fb11.ssh.shell = 'sh'
    #     fb11.vm.provider 'virtualbox' do |vb|
    #         vb.name = 'fb11_circonus-agent_build'
    #         vb.customize ['modifyvm', :id, '--memory', '2048']
    #         vb.customize ['modifyvm', :id, '--cpus', '2']
    #         vb.customize ['modifyvm', :id, '--hwvirtex', 'on']
    #         vb.customize ['modifyvm', :id, '--audio', 'none']
    #         vb.customize ['modifyvm', :id, '--nictype1', 'virtio']
    #         vb.customize ['modifyvm', :id, '--nictype2', 'virtio']
    #     end
    #     fb11.vm.network 'private_network', ip: '192.168.100.241'
    #     fb11.vbguest.auto_update = false
    #     fb11.vm.provision 'bootstrap', type: 'shell', inline: <<-SHELL
    #         echo "Installing needed packages (e.g. git, gcc, etc.)"
    #         pkg install -y -q git gcc gmake bash logrotate curl
    #         chsh -s /usr/local/bin/bash vagrant
    #     SHELL
    #     fb11.vm.provision 'shell', path: 'vprov/fb11.sh', args: [go_url_base, go_ver]
    #     fb11.trigger.after(:halt) do |trigger|
    #         trigger.info = "Purging NFSD exports"
    #         trigger.ruby do |_env, _machine|
    #             etc_exports = Pathname.new("/etc/exports")
    #             if etc_exports.exist? && etc_exports.size > 0
    #                 system(%q{sudo cp /dev/null /etc/exports && sudo nfsd restart})
    #             end
    #         end
    #     end
    # end

end
