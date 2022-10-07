#!/usr/bin/env bash

go_base_url="$1"
go_ver="$2"
go_install=0
go_tgz=""
os_type=$(uname -s)
# make this simple since we only do linux and freebsd builds
if [[ ${os_type,,} == "linux" ]]; then
	go_tgz="${go_ver}.linux-amd64.tar.gz"
else
	go_tgz="${go_ver}.freebsd-amd64.tar.gz"
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
