#!/usr/local/bin/bash

go_base_url="$1"
go_ver="$2"

if [[ $(grep -c fdescfs /etc/fstab) -eq 0 ]]; then
    mount -t fdescfs fdescfs /dev/fd
    echo 'fdescfs	/dev/fd		fdescfs		rw,late	0	0' >> /etc/fstab
fi

if [[ ! -x /usr/local/go/bin/go ]]; then
    go_tgz="go${go_ver}.freebsd-amd64.tar.gz"
    [[ -f /vagrant/${go_tgz} ]] || {
        go_url="${go_base_url}/${go_tgz}"
        echo "Downloading ${go_url}"
        curl -sSL "$go_url" -o /home/vagrant/$go_tgz
        [[ $? -eq 0 ]] || { echo "Unable to download go tgz"; exit 1; }
    }
    tar -C /usr/local -xf /home/vagrant/$go_tgz
    [[ $? -eq 0 ]] || { echo "Error unarchiving $go_tgz"; exit 1; }
fi

bashprofile=~vagrant/.bash_profile
[[ -f $bashprofile ]] || cat > $bashprofile <<"EBP"
#created by vagrant
[[ -f $HOME/.bashrc ]] && source $HOME/.bashrc
EBP
bashrc=~vagrant/.bashrc
[[ -f $bashrc ]] || echo "#created by vagrant" > $bashrc
[[ $(grep -c "bin/go" $bashrc) -eq 0 ]] && echo 'export PATH="${PATH}:/usr/local/go/bin"' >> $bashrc
[[ $(grep -c GOPATH $bashrc) -eq 0 ]] && echo 'export GOPATH="${HOME}/godev"' >> $bashrc
chown vagrant:vagrant ~vagrant/.{bashrc,bash_profile}
chmod 600 ~vagrant/.{bashrc,bash_profile}
chown vagrant:vagrant ~vagrant/godev
