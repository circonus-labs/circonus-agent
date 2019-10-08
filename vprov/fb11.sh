#!/usr/local/bin/bash

go_base_url="$1"
go_ver="$2"
go_install=0

SUDO="sudo"
[[ "$(id -u)" == "0" ]] && SUDO=""

if [[ $(grep -c fdescfs /etc/fstab) -eq 0 ]]; then
    $SUDO mount -t fdescfs fdescfs /dev/fd
    $SUDO sh -c "echo 'fdescfs	/dev/fd		fdescfs		rw,late	0	0' >> /etc/fstab"
fi


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
    go_tgz="${go_ver}.freebsd-amd64.tar.gz"
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

bashprofile=~vagrant/.bash_profile
[[ -f $bashprofile ]] || cat > $bashprofile <<"EBP"
#created by vagrant
[[ -f $HOME/.bashrc ]] && source $HOME/.bashrc
EBP
bashrc=~vagrant/.bashrc
[[ -f $bashrc ]] || echo "#created by vagrant" > $bashrc
[[ $(grep -c "go/bin" $bashrc) -eq 0 ]] && echo 'export PATH="${PATH}:/usr/local/go/bin"' >> $bashrc
[[ $(grep -c GOPATH $bashrc) -eq 0 ]] && echo 'export GOPATH="${HOME}/godev"' >> $bashrc
chown vagrant:vagrant ~vagrant/.{bashrc,bash_profile}
chmod 600 ~vagrant/.{bashrc,bash_profile}
chown vagrant:vagrant ~vagrant/godev

echo "provisioning complete"
# DONE