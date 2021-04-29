#!/usr/bin/env bash

###
### !! IMPORTANT !!
###
### Requires VMs for target OSes due to c plugins and cgo in protocol_observer.
### The actual agent is cross-compiled and stored in releases within the github repo.
###
### See os provisioning sections in Vagrantfile for packages required to build.
###

[[ "$*" =~ (-d|--debug) ]] && set -o xtrace
set -o errtrace
set -o errexit
set -o nounset

umask 0022

# load standard build overrides
[[ -f ./build.conf ]] && source ./build.conf

#
# NOTE: to pull down alternate forks use build.conf to override the explicit
#       repo url - *do not modify* these variables
agent_name="circonus-agent"
plugins_name="circonus-agent-plugins"
po_name="wirelatency"
logwatch_name="circonus-logwatch"
base_repo_url="https://github.com/circonus-labs"

: ${dir_build:="/tmp/agent-build"}
: ${dir_install:="/tmp/agent-install"}
: ${dir_publish:="$(pwd)/publish"}

# NOTE: the publish directory should ALREADY exist
[[ -d $dir_publish ]] || { echo "publish directory ($dir_publish) not found!"; exit 1; }

dir_agent_build="${dir_build}/${agent_name}"
dir_plugin_build="${dir_build}/${plugins_name}"
dir_po_build="${dir_build}/${po_name}"
dir_logwatch_build="${dir_build}/${logwatch_name}"

#
# settings which can be overridden in build.conf
#

# NOTE: Repos are tag driven, this allows master to be out of sync with
#       the release cadence and enables reproducing a previous release.
#       To this end, there are some rules which must be adhered to for this
#       to all work correctly.

# NOTE: circonus-agent version must be 'latest', 'snapshot', or a specific release tag.
#       It cannot be 'master' or a branch name. See 'goreleaser' below.
: ${agent_version:="latest"}
: ${logwatch_version:="latest"} # same caveat as agent
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
# same caveat as agent
: ${url_logwatch_repo:="${base_repo_url}/${logwatch_name}"}

# Using a fork for plugins is more straight-forward. Fork, change, set
# plugin_version in build.conf to 'master' and build the package.
: ${url_plugin_repo:="${base_repo_url}/${plugins_name}"}
# same caveat as plugin
: ${url_po_repo:="${base_repo_url}/${po_name}"}

: ${dir_install_prefix:="/opt/circonus"}
: ${dir_install_agent:="${dir_install}${dir_install_prefix}/agent"}
: ${dir_install_logwatch:="${dir_install}${dir_install_prefix}/logwatch"}

#
# commands used during build/install
#
: ${CP:="cp"}
: ${CP_ARGS:="-v"}
: ${CURL:="curl"}
: ${GIT:="git"}
: ${GO:="go"} # for protocol_observer (requires cgo)
: ${MKDIR:="mkdir"}
: ${RM:="rm"}
: ${SED:="sed"}
: ${TAR:="tar"}
: ${TR:="tr"}
: ${UNAME:="uname"}
for cmd in $CP $CURL $GIT $GO $MKDIR $RM $SED $TAR $TR $UNAME; do
    [[ -z "$(type -P $cmd)" ]] && { echo "unable to find '${cmd}' command in [$PATH]"; exit 1; }
done
: ${FPM:="/usr/local/bin/fpm"}  # only used by Ubuntu builds
: ${MAKE:="make"}               # freebsd alters
: ${RPMBUILD:="rpmbuild"}       # only used by RHEL builds

os_type=$($UNAME -s | $TR '[:upper:]' '[:lower:]')
os_arch=$($UNAME -m)
[[ $os_arch =~ ^(x86_64|amd64)$ ]] || { echo "unsupported architecture ($os_arch) - x86_64 or amd64 only"; exit 1; }
os_name=""
install_target=""
agent_tgz=""
agent_tgz_url=""
logwatch_tgz=""
logwatch_tgz_url=""
case $os_type in
    linux)
        if [[ -f /etc/redhat-release ]]; then
            install_target="install-rhel"
            relver=$(sed -e 's/.*release \(.\).*/\1/' /etc/redhat-release)
            [[ $relver =~ ^(6|7|8)$ ]] || { echo "unsupported RHEL release ($relver)"; exit 1; }
            os_name="el${relver}"
            [[ -z "$(type -P $RPMBUILD)" ]] && { echo "unable to find '${RPMBUILD}' command in [$PATH]"; exit 1; }
            [[ -d ~/rpmbuild/RPMS ]] || { echo "~/rpmbuild/RPMS not found, is rpm building setup?"; exit 1; }
        elif [[ -f /etc/lsb-release ]]; then
            install_target="install-ubuntu"
            source /etc/lsb-release
            [[ $DISTRIB_RELEASE =~ ^(14|16|18|20)\.04$ ]] || { echo "unsupported Ubuntu release ($DISTRIB_RELEASE)"; exit 1; }
            os_name="ubuntu.${DISTRIB_RELEASE}"
            [[ -z "$(type -P $FPM)" ]] && { echo "unable to find '${FPM}' command in [$PATH]"; exit 1; }
        else
            echo "unknown/unsupported linux variant '$($UNAME -a)'"
            exit 1
        fi
        ;;
    #
    # TODO: Should add omnios/illumos package building here so there is ONE place
    #       where packages are built, rather than two which need to be kept in sync?
    #       Or, move all of this to the official "packaging" repository for the same
    #       "single source of truth" outcome. Note, the cadence for the agent will,
    #       at times, be higher than for the "all of circonus" packaging cadence.
    #       Or, build an solaris tgz here and then use the rpm/deb/tgz in the
    #       'master' product package builder.
    #       -- point being to have a single source controlling the build to eliminate divergence
    #
    freebsd)
        install_target="install-freebsd"
        relver=$(freebsd-version -u | cut -d'-' -f1)
        [[ -z $relver ]] && { echo "unsupported FreeBSD release >10 required"; exit 1; }
        os_name="$os_type.$relver"
        MAKE="gmake"
        ;;
    *)
        echo "unknown/unsupported OS ($os_type)"
        exit 1
        ;;
esac

[[ -z "$(type -P $MAKE)" ]] && { echo "unable to find '${MAKE}' command in [$PATH]"; exit 1; }

[[ -z "$os_name" ]] && { echo "invalid os_name (empty)"; exit 1; }
[[ -z "$install_target" ]] && { echo "invalid install_target (empty)"; exit 1; }

###
### start building package
###

echo
echo "Building circonus-agent package for ${os_name} ${os_arch}"
echo

[[ -d $dir_build ]] || { echo "-creating build directory"; $MKDIR -p $dir_build; }
[[ -d $dir_install ]] && { echo "-cleaning previous install directory"; $RM -rf $dir_install; }
[[ -d $dir_install ]] || { echo "-creating install directory"; $MKDIR -p $dir_install; }

##
## Agent: use pre-built release package (single source of truth...)
##
fetch_agent_repo() {
    if [[ -d $dir_agent_build ]]; then
        echo "-updating agent repo"
        pushd $dir_agent_build >/dev/null
        $GIT checkout master # ensure on master branch
        $GIT pull
        popd >/dev/null
    else
        pushd $dir_build >/dev/null
        echo "-cloning agent repo"
        local url_repo=${url_agent_repo/#https/git}
        $GIT clone $url_repo
        popd >/dev/null
    fi

    if [[ "$agent_version" == "latest" ]]; then
        # get latest released tag otherwise _assume_ it is set to a
        # valid tag version in the repository or master.
        pushd $dir_agent_build >/dev/null
        agent_version=$($GIT describe --abbrev=0 --tags)
        popd >/dev/null
    fi
    echo "-using agent version ${agent_version}"

    dir_publish="${dir_publish}/${agent_version}"
    [[ -d $dir_publish ]] || $MKDIR -p $dir_publish
    echo "publishing packing to: ${dir_publish}"
}
fetch_agent_package() {
    if [[ "$agent_version" != "snapshot" ]]; then
        local stripped_ver=${agent_version#v}
        agent_tgz="${agent_name}_${stripped_ver}_${os_type}_x86_64.tar.gz"
        agent_tgz_url="${url_agent_repo}/releases/download/${agent_version}/$agent_tgz"
        [[ -f $agent_tgz ]] || {
            echo "-fetching agent package (${agent_tgz}) - ${agent_tgz_url}"
            $CURL -fsSL "$agent_tgz_url" -o $agent_tgz
        }
    else
        dir_dist="${dir_current}/../dist"
        [[ -d $dir_dist ]] || { echo "'dist' directory (${dir_dist}) not found"; exit 1; }
        agent_tgz=$(ls ${dir_dist}/circonus-agent*${os_type}_${os_arch}.tar.gz)
        [[ $? -eq 0 ]] || { echo "unable to find snapshot in ../dist"; exit 1; }
        [[ -f $agent_tgz ]] || { echo "unable to isolate ONE snapshot file (${agent_tgz})"; exit 1; }
    fi
}
install_agent() {
    echo
    echo "Installing circonus-agent from ${url_agent_repo}"
    echo

    pushd $dir_build >/dev/null
    fetch_agent_repo
    fetch_agent_package
    echo "-unpacking $agent_tgz into $dir_install_agent"
    [[ -d $dir_install_agent ]] || $MKDIR -p $dir_install_agent
    $TAR -xf $agent_tgz -C $dir_install_agent
    popd >/dev/null
}

##
## plugins are target built
##
fetch_plugin_repo() {
    if [[ -d $dir_plugin_build ]]; then
        echo "-updating plugin repo"
        pushd $dir_plugin_build >/dev/null
        $GIT checkout master # ensure on master branch
        $GIT pull
        popd >/dev/null
    else
        pushd $dir_build >/dev/null
        echo "-cloning plugin repo"
        local url_repo=${url_plugin_repo/#https/git}
        $GIT clone $url_repo
        popd >/dev/null
    fi

    if [[ "$plugin_version" == "latest" ]]; then
        # get latest released tag otherwise _assume_ it is set to a
        # valid tag version in the repository or master.
        pushd $dir_plugin_build >/dev/null
        plugin_version=$($GIT describe --abbrev=0 --tags)
        popd >/dev/null
    fi
    echo "-using plugin version ${plugin_version}"
}
install_plugins() {
    echo
    echo "Installing circonus-agent-plugins from ${url_plugin_repo}"
    echo

    fetch_plugin_repo

    pushd $dir_plugin_build >/dev/null
    $GIT checkout master # ensure on master branch
    [[ $plugin_version == "master" ]] || $GIT checkout tags/$plugin_version
    $MAKE DESTDIR=$dir_install $install_target
    popd >/dev/null
}

##
## protocol_observer needs to be target built due to cgo
##
fetch_protocol_observer_repo() {
    if [[ -d $dir_po_build ]]; then
        echo "-updating wirelatency [protocol_observer] repo"
        pushd $dir_po_build >/dev/null
        $GIT checkout master # ensure on master branch
        $GIT pull
        popd >/dev/null
    else
        pushd $dir_build >/dev/null
        echo "-cloning wirelatency [protocol_observer] repo"
        local url_repo=${url_po_repo/#https/git}
        $GIT clone $url_repo
        popd >/dev/null
    fi

    if [[ "$po_version" == "latest" ]]; then
        # get latest released tag otherwise _assume_ it is set to a
        # valid tag version in the repository or master.
        pushd $dir_po_build >/dev/null
        po_version=$($GIT describe --abbrev=0 --tags)
        popd >/dev/null
    fi
    echo "-using wirelatency [protocol_observer] version ${po_version}"
}
install_protocol_observer() {
    echo
    echo "Installing protocol_observer from ${url_po_repo}"
    echo

    local rm_go_mod="n"
    local dest_bin="${dir_install_agent}/sbin/protocol-observerd"
    local dest_doc="${dir_install_agent}/README_protocol-observer.md"
    fetch_protocol_observer_repo

    pushd $dir_po_build >/dev/null
    $GIT checkout master # ensure on master branch
    [[ $po_version == "master" ]] || $GIT checkout tags/$po_version
    #
    # NOTE: protocol_observer is a tool in the wirelatency repository
    #
    pushd protocol_observer >/dev/null
    echo "-building protocol_observer (${dest_bin})"
    $GO build -o $dest_bin
    echo "-installed ${dest_bin}"
    [[ -f README.md ]] && {
        echo "-installing protocol_observer doc (${dest_doc})"
        $CP $CP_ARGS README.md $dest_doc
    }
    popd >/dev/null
    popd >/dev/null
}

##
## Logwatch: use pre-built release package (single source of truth...)
##
fetch_logwatch_repo() {
    if [[ -d $dir_logwatch_build ]]; then
        echo "-updating logwatch repo"
        pushd $dir_logwatch_build >/dev/null
        $GIT checkout master # ensure on master branch
        $GIT pull
        popd >/dev/null
    else
        pushd $dir_build >/dev/null
        echo "-cloning logwatch repo"
        local url_repo=${url_logwatch_repo/#https/git}
        $GIT clone $url_repo
        popd >/dev/null
    fi

    if [[ "$logwatch_version" == "latest" ]]; then
        # get latest released tag otherwise _assume_ it is set to a
        # valid tag version in the repository or master.
        pushd $dir_logwatch_build >/dev/null
        logwatch_version=$($GIT describe --abbrev=0 --tags)
        popd >/dev/null
    fi
    echo "-using logwatch version ${logwatch_version}"
}
fetch_logwatch_package() {
    local stripped_ver=${logwatch_version#v}
    logwatch_tgz="${logwatch_name}_${stripped_ver}_${os_type}_x86_64.tar.gz"
    logwatch_tgz_url="${url_logwatch_repo}/releases/download/${logwatch_version}/$logwatch_tgz"
    [[ -f $logwatch_tgz ]] || {
        echo "-fetching logwatch package (${logwatch_tgz}) - ${logwatch_tgz_url}"
        $CURL -fsSL "$logwatch_tgz_url" -o $logwatch_tgz
    }
}
install_logwatch() {
    #
    # currently logwatch is only built for linux and freebsd
    #
    if [[ $os_type =~ (linux|freebsd) ]]; then
        echo
        echo "Installing circonus-logwatch from ${url_logwatch_repo}"
        echo

        pushd $dir_build >/dev/null
        fetch_logwatch_repo
        fetch_logwatch_package

        # TODO: add service config examples (at least systemd) to logwatch

        echo "-unpacking $logwatch_tgz into $dir_install_logwatch"
        [[ -d $dir_install_logwatch ]] || $MKDIR -p $dir_install_logwatch
        $TAR -xf $logwatch_tgz -C $dir_install_logwatch
        popd >/dev/null
    fi
}

##
## install os specific service configuration(s)
##
install_service() {
    echo
    echo "Installing circonus-agent service configuration"
    echo

    local sed_script="s#@@SBIN@@#${dir_install_prefix}/agent/sbin#"

    # NOTE: just copy the file, let packaging handle perms
    case $os_name in
        el8)
            $MKDIR -p $dir_install/lib/systemd/system
            $SED -e "${sed_script}" ../service/circonus-agent.service > $dir_install/lib/systemd/system/circonus-agent.service
            ;;
        el7)
            $MKDIR -p $dir_install/lib/systemd/system
            $SED -e "${sed_script}" ../service/circonus-agent.service > $dir_install/lib/systemd/system/circonus-agent.service
            ;;
        el6)
            $MKDIR -p $dir_install/etc/init.d
            $SED -e "${sed_script}" ../service/circonus-agent.init-rhel > $dir_install/etc/init.d/circonus-agent
            ;;
        ubuntu\.20*)
            $MKDIR -p $dir_install/lib/systemd/system
            $SED -e "${sed_script}" ../service/circonus-agent.service > $dir_install/lib/systemd/system/circonus-agent.service
            ;;
        ubuntu\.1[68]*)
            $MKDIR -p $dir_install/lib/systemd/system
            $SED -e "${sed_script}" ../service/circonus-agent.service > $dir_install/lib/systemd/system/circonus-agent.service
            ;;
        ubuntu\.14*)
            $MKDIR -p $dir_install/etc/init.d
            $SED -e "${sed_script}" ../service/circonus-agent.init-ubuntu > $dir_install/etc/init.d/circonus-agent
            chmod 755 $dir_install/etc/init.d/circonus-agent
            ;;
        freebsd\.*)
            $MKDIR -p $dir_install/etc/rc.d
            if [[ -d ../service ]]; then
                $SED -e "$sed_script" ../service/circonus-agent.rc-freebsd > $dir_install/etc/rc.d/circonus-agent
                chmod 755 $dir_install/etc/rc.d/circonus-agent
            elif [[ -f ../service ]]; then
                $SED -e "$sed_script" ../service > $dir_install/etc/rc.d/circonus-agent
                chmod 755 $dir_install/etc/rc.d/circonus-agent
            else
                echo "unable to install service ${PWD}../service is not a file or directory"
                exit 1
            fi
            ;;
        *)
            echo "no pre-built service configuration available for $os_name"
            ;;
    esac
}

##
## build the target package
##
make_package() {
    local stripped_ver=${agent_version#v}

    #
    # deb/rpm treat version numbers differently
    # semver compliant pre-release versions fail to build or parse differently than intended
    # to circumvent the build/parse issues the dash will be replaced with a tilde (until 
    # the semver standard is updated with a course of action)
    #

    echo
    echo "Creating circonus-agent package"
    echo

    #
    # remove the pre-built linux io latency binary if not on a linux variant
    # TODO: migrate io latency plugin from circonus-agent repo to circonus-agent-plugins repo
    #
    [[ $os_type != "linux" && -d $dir_install_agent/plugins/linux ]] && $RM -rf $dir_install_agent/plugins/linux

    case $os_name in
        el*)
            echo "making RPM for $os_name"
            stripped_ver="${stripped_ver/-/~}"
            $SED -e "s#@@RPMVER@@#${stripped_ver}#" rhel/circonus-agent.spec.in > rhel/circonus-agent.spec
            $RPMBUILD -bb rhel/circonus-agent.spec
            $CP $CP_ARGS ~/rpmbuild/RPMS/*/circonus-agent-${stripped_ver}-1.*.${os_arch}.rpm $dir_publish
            $RM rhel/circonus-agent.spec
            ;;
        ubuntu*)
            echo "making DEB for $os_name"
            deb_file="${dir_build}/circonus-agent-${stripped_ver}-1.${os_name}_${os_arch}.deb"

            if [[ -f $deb_file ]]; then
                echo "-found previous ${deb_file}, removing file"
                $RM -f $deb_file
            fi

            chown nobody:nobody $dir_install_agent/etc && chmod 0750 $dir_install_agent/etc

            # when snapshots are used, the embedded Version field in the deb needs
            # to start with a number. fudge it since these are !!ONLY!! for testing.
            [[ "$agent_version" == "snapshot" ]] && stripped_ver="1.${agent_version}"

            $FPM -s dir \
                -t deb \
                -n circonus-agent \
                -v $stripped_ver \
                --iteration 1 \
                -C $dir_install \
                -p $deb_file \
                --url "$url_agent_repo" \
                --vendor "Circonus, Inc." \
                --license "BSD" \
                --maintainer "Circonus Support <support@circonus.com>" \
                --description "Circonus agent daemon" \
                --deb-no-default-config-files \
                --deb-user root \
                --deb-group root \
                --after-install ${PWD}/ubuntu/postinstall.sh \
                --after-remove ${PWD}/ubuntu/postremove.sh
            $CP $CP_ARGS $deb_file $dir_publish
            $RM $deb_file
            ;;
        *)
            pushd $dir_install >/dev/null
            chown nobody:nobody $dir_install_agent/etc && chmod 0750 $dir_install_agent/etc
            echo "making tgz for $os_name"
            local pkg="${dir_build}/circonus-agent-${stripped_ver}-1.${os_name}_${os_arch}.tgz"
            $TAR czf $pkg .
            $CP $CP_ARGS $pkg $dir_publish
            $RM $pkg
            popd >/dev/null
            ;;
    esac
}

#
## creating a circonus-agent package
#
dir_current=$(pwd)
install_agent
install_plugins
# removing inclusion of protocol_observer temporarily 
# so it stops blocking package builds due to protocol
# changes and changes to gopacket preventing compiling.
# to re-enable inclusion, uncomment the following line
# and remove this comment.
#install_protocol_observer
install_logwatch
install_service
make_package

# Vim hints
# vim:ts=4:sw=4:et:
