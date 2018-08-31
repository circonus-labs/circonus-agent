#!/usr/bin/env bash

set -o errtrace
set -o errexit
set -o nounset
umask 0022

# load standard build overrides
[[ -f ./build.conf ]] && source ./build.conf

#
# NOTE: to pull down alternate forks use build.conf to override the explicit
#       repo url - do not modify these variables
agent_name="circonus-agent"
plugins_name="circonus-agent-plugins"
po_name="wirelatency"
base_repo_url="https://github.com/circonus-labs"

: ${dir_build:="/tmp/agent-build"}
: ${dir_install:="/tmp/agent-install"}
: ${dir_publish:="/tmp/agent-publish"}
: ${dir_agent_build:="${dir_build}/${agent_name}"}
: ${dir_plugin_build:="${dir_build}/${plugins_name}"}
: ${dir_po_build:="${dir_build}/${po_name}"}

#
# settings which can be overridden in build.conf
#

# NOTE: Repos are tag driven, this allows master to be out of sync with
#       the release cadence and enables reproducing a previous released.
#       To this end, there are some rules which must be adhered to for this
#       to all work correctly.

# NOTE: circonus-agent version must be 'latest' or a specific release tag.
#       It cannot be 'master' or a branch name. See 'goreleaser' below.
: ${agent_version:="latest"}
: ${plugin_version:="latest"} # 'latest', 'master', or specific tag
: ${po_version:="latest"} # same caveat as plugin

# NOTE: goreleaser is used to produce cross-compiled binaries in the
#       official circonus-labs/circonus-agent repository.
#
# In order to use a fork of circonus-agent, goreleaser needs to be configured
# to publish releases into the fork **DO NOT COMMIT AN ALTERED .goreleaser.yml**.
# The master config should **always** point to _circonus-labs/circonus-agent_!
# The steps would be fork, update .goreleaser.yml to point to your fork, make
# changes, commit, tag, run goreleaser to produce a "release" which can be used
# to build a package for testing.
: ${url_agent_repo:="${base_repo_url}/${agent_name}"}
# Using a fork for plugins is more straight-forward. Fork, change, set
# plugin_version in build.conf to 'master' and build the package.
: ${url_plugin_repo:="${base_repo_url}/${plugins_name}"}
# same caveat as plugin
: ${url_po_repo}:="${base_repo_url}/${po_name}"

: ${dir_install_prefix:="/opt/circonus"}
: ${dir_install_agent:="${dir_install_prefix}/agent"}

#
# commands used during build/install
#
: ${CP:="cp"}
: ${CURL:="curl"}
: ${GIT:="git"}
: ${GO:="go"} # for protocol_observer
: ${MKDIR:="mkdir"}
: ${RM:="rm"}
: ${TAR:="tar"}
: ${TR:="tr"}
: ${UNAME:="uname"}
for cmd in $CURL $GIT $TR $UNAME; do {
    [[ -z "$(type -P $cmd)" ]] && { echo "unable to find '${cmd}' command in [$PATH]"; exit 1}
}
: ${FPM:="/usr/local/bin/fpm"}  # only used by Ubuntu builds
: ${MAKE:="make"}               # freebsd alters
: ${RPMBUILD:="rpmbuild"}       # only used by RHEL builds

os_type=$($UNAME -s | $TR '[:upper:]' '[:lower:]')
os_arch=$($UNAME -m)
[[ $os_arch =~ ^(x86_64|amd64)$ ]] || { echo "unsupported architecture ($os_arch) - x86_64 or amd64 only"; exit 1; }

# check for custom target os overrides (e.g. build-linux.conf)
cust_conf="build-${os_type}.conf"
[[ -f $cust_conf ]] && source ./$cust_conf

os_name=""
install_target=""
package_name=""
case $os_type in
    linux)
        if [[ -f /etc/redhat-release ]]; then
            install_target="install-rhel"
            relver=$(sed -e 's/.*release \(.\).*/\1/' /etc/redhat-release)
            [[ $relver =~ ^(6|7)$ ]] || { echo "unsupported RHEL release ($relver)"; exit 1; }
            os_name="el${relver}"
            package_name="${agent_name}-${agent_version}-1.${os_name}_${os_arch}.rpm"
            [[ -z "$(type -P $RPMBUILD)" ]] && { echo "unable to find '${RPMBUILD}' command in [$PATH]"; exit 1; }
            [[ -d ~/rpmbuild/RPMS ]] || { echo "~/rpmbuilds/RPMS not found, is rpm building setup?"; exit 1; }
        elif [[ -f /etc/lsb-release ]]; then
            install_target="install-ubuntu"
            source /etc/lsb-release
            [[ $DISTRIB_RELEASE =~ ^(14.04|16.04)$ ]] || { echo "unsupported Ubuntu release ($DISTRIB_RELEASE)"; exit 1; }
            os_name="ubuntu.${DISTRIB_RELEASE}"
            package_name="${agent_name}-${agent_version}-1.${os_name}_${os_arch}.deb"
            [[ -z "$(type -P $FPM)" ]] && { echo "unable to find '${FPM}' command in [$PATH]"; exit 1}
        else
            echo "unknown/unsupported linux variant '$($UNAME -a)'"
            exit 1
        fi
        ;;
    freebsd)
        install_target="install-freebsd"
        relver=`freebsd-version -u | cut -d'-' -f1`
        [[ -z $relver ]] && { echo "unsupported FreeBSD release >10 required"; exit 1; }
        os_name="$os_type.$relver"
        package_name="${agent_name}-${agent_version}-1.${os_name}_${os_arch}.tgz"
        MAKE="gmake"
        ;;
    *)
        echo "unknown/unsupported OS ($os_type)"
        exit 1
        ;;
esac

[[ -z "$(type -P $MAKE)" ]] && { echo "unable to find '${MAKE}' command in [$PATH]"; exit 1}

[[ -z "$os_name" ]] && { echo "invalid os_name (empty)"; exit 1; }
[[ -z "$package_name" ]] && { echo "invalid package_name (empty)"; exit 1; }
[[ -z "$install_target" ]] && { echo "invalid install_target (empty)"; exit 1; }

if [[ -x /usr/bin/nproc ]]; then
    nproc=$(nproc)
elif [[ -f /proc/cpuinfo ]]; then
    nproc=$(grep -c ^processor /proc/cpuinfo)
else
    nproc=`sysctl -n hw.ncpu`
fi
let make_jobs="$nproc + ($nproc / 2)" # 1.5x the number of CPUs

agent_tgz=""
agent_tgz_url=""

###
### start building package
###

echo "Building for ${os_name} ${os_arch}"
echo

[[ -d $dir_build ]] || {
    echo "-creating build directory"
    $MKDIR -p $dir_build
}
[[ -d $dir_install ]] && {
    echo "-cleaning previous install directory"
    $RM -rf $dir_install
}
[[ -d $dir_install ]] || {
    echo "-creating install directory"
    $MKDIR -p $dir_install
}

##
## use pre-built agent release packages (single source of truth...)
##
fetch_agent_repo() {
    if [[ -d $dir_agent_build ]]; then
        echo "-updating agent repo"
        pushd $dir_agent_build >/dev/null
        $GIT checkout master # ensure on master branch
        $GIT pull
        popd >/dev/null
    else
        echo "-cloning agent repo"
        local url_repo=${url_agent_repo/#https/git}
        $GIT clone $url_repo
    fi

    if [[ "$agent_version" == "latest" ]]; then
        # get latest released tag otherwise _assume_ it is set to a
        # valid tag version in the repository.
        pushd $dir_agent_build >/dev/null
        agent_version=$($GIT describe --abbrev=0 --tags)
        echo "-using agent version ${agent_version}"
        popd >/dev/null
    fi
}
fetch_agent_package() {
    local stripped_ver=${agent_ver#v}
    agent_tgz="${agent_name}_${stripped_ver}_${os_type}_64-bit.tar.gz"
    agent_tgz_url="${url_agent_repo}/releases/download/${agent_version}/$agent_tgz"
    [[ -f $agent_tgz ]] || {
        echo "-fetching agent package (${agent_tgz}) - ${agent_tgz_url}"
        $CURL -sSL "$agent_tgz_url" -o $agent_tgz
    }
}
install_agent() {
    fetch_agent_repo
    fetch_agent_package

    echo "-unpacking $agent_tgz into $dir_install_agent"
    $TAR -zxf $agent_tgz -C $dir_install_agent
}

##
## make target specific plugins
##
fetch_plugin_repo() {
    if [[ -d $dir_plugin_build ]]; then
        echo "-updating plugin repo"
        pushd $dir_plugin_build >/dev/null
        $GIT checkout master # ensure on master branch
        $GIT pull
        popd >/dev/null
    else
        echo "-cloning plugin repo"
        local url_repo=${url_plugin_repo/#https/git}
        $GIT clone $url_repo
    fi

    if [[ "$plugin_version" == "latest" ]]; then
        # get latest released tag otherwise _assume_ it is set to a
        # valid tag version in the repository or master.
        pushd $dir_plugin_build >/dev/null
        plugin_version=$($GIT describe --abbrev=0 --tags)
        echo "-using plugin version ${plugin_version}"
        popd >/dev/null
    fi
}
install_plugins() {
    fetch_plugin_repo

    pushd $dir_plugin_build >/dev/null
    [[ $plugin_version == "master" ]] || $GIT checkout tags/$plugin_version
    $MAKE DEST=$dir_install_agent $install_target
    popd >/dev/null
}

##
## use pre-built protocol_observer release packages (single source of truth...)
##
fetch_protocol_observer_repo() {
    if [[ -d $dir_po_build ]]; then
        echo "-updating wirelatency repo"
        pushd $dir_po_build >/dev/null
        $GIT checkout master # ensure on master branch
        $GIT pull
        popd >/dev/null
    else
        echo "-cloning wirelatency repo"
        local url_repo=${url_po_repo/#https/git}
        $GIT clone $url_repo
    fi

    if [[ "$po_version" == "latest" ]]; then
        # get latest released tag otherwise _assume_ it is set to a
        # valid tag version in the repository.
        pushd $dir_po_build >/dev/null
        po_version=$($GIT describe --abbrev=0 --tags)
        echo "-using protocol_observer version ${po_version}"
        popd >/dev/null
    fi
}
install_protocol_observer() {
    fetch_protocol_observer_repo

    pushd $dir_po_build/protocol_observer >/dev/null
    [[ $po_version == "master" ]] || $GIT checkout tags/$po_version
    $GO build -o ${dir_install_agent}/sbin/protocol-observerd
    popd >/dev/null
}

##
## build the target package
##
make_package() {
    case $os_name in
        el*)
            pushd $dir_agent_build >/dev/null
            echo "making RPM for $os_name ($package_name)"
            popd >/dev/null
            ;;
        ubuntu*)
            pushd $dir_agent_build >/dev/null
            echo "making DEB for $os_name ($package_name)"
            popd >/dev/null
            ;;
        *)
            pushd $dir_install >/dev/null
            echo "making tgz for $os_name ($package_name)"
            $TAR czf $package_name .
            $CP $package_name $dir_publish
            $RM $package_name
            popd >/dev/null
            ;;
    esac

    echo
    echo "NOT IMPLEMENTED YET"
    echo
}

[[ -f "${dir_publish}/${package_name}" ]] && {
    echo "package ($package_name) already exists SKIPPING build"
    exit 0
}

pushd $dir_build >/dev/null

install_agent
install_plugins
install_protocol_observer
make_package

popd >/dev/null

# Vim hints
# vim:ts=4:sw=4:et:
