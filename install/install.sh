#!/usr/bin/env bash

set -o errtrace
set -o errexit
set -o nounset

# ignore tput errors for terms that do not
# support colors (colors will be blank strings)
set +e
RED=$(tput setaf 1)
GREEN=$(tput setaf 2)
NORMAL=$(tput sgr0)
BOLD=$(tput bold)
set -e

ca_version=""
pkg_arch="x86_64" # currently: only x86_64
pkg_ext=""        # currently: rpm or deb
pkg_cmd=""        # currently: yum or dpkg
pkg_args=""
pkg_file=""
pkg_url=""
ca_broker_id=""
ca_api_key=""
ca_api_app=""
ca_os_sig=""
ca_conf_file="/opt/circonus/agent/etc/circonus-agent.yaml"

usage() {
  printf "%b" "Circonus Agent Install Help

Usage

  ${GREEN}install.sh --key <apikey>${NORMAL}

Options

  --key           Circonus API key/token **${BOLD}REQUIRED${NORMAL}**
  [--app]         Circonus API app name (authorized w/key) Default: circonus-agent
  [--broker]      Circonus Broker ID (_cid from broker api object|select) Default: select
  [--ver]         Circonus Agent version tag (e.g. v2.3.2) Default: latest release
  [--os]          Install for specific os if agent unable to detect 
                  (el8|el7|el6|ubuntu.20.04|ubuntu.18.04|ubuntu.16.04)
  [--help]        This message

Note: Provide an authorized app for the key or ensure api 
      key/token has adequate privileges (default app state:allow)
"
}

log()  { printf "%b\n" "$*"; }
fail() { printf "${RED}" >&2; log "\nERROR: $*\n" >&2; printf "${NORMAL}" >&2; exit 1; }
pass() { printf "${GREEN}"; log "$*"; printf "${NORMAL}"; }

__parse_parameters() {
    local token=""
    log "Parsing command line parameters"
    while (( $# > 0 )) ; do
        token="$1"
        shift
        case "$token" in
        (--key)
            if [[ -n "${1:-}" ]]; then
                ca_api_key="$1"
                shift
            else
                fail "--key must be followed by an api key."
            fi
            ;;
        (--app)
            if [[ -n "${1:-}" ]]; then
                ca_api_app="$1"
                shift
            else
                fail "--app must be followed by an api app."
            fi
            ;;
        (--ver)
            if [[ -n "${1:-}" ]]; then
                ca_version="$1"
                shift
            else
                fail "--ver must be followed by a release tag."
            fi
            ;;
        (--broker)
            if [[ -n "${1:-}" ]]; then
                ca_broker_id="$1"
                shift
            else
                fail "--broker must be followed by Broker Group ID."
            fi
            ;;
        (--is)
            if [[ -n "${1:-}" ]]; then
                ca_os_sig="$1"
                shift
            else
                fail "--os must be followed by an os signature."
            fi
            ;;
        esac
    done
}

__ca_init() {
    set +o errexit
    
    # trigger error if needed commands are not found...
    local cmd_list="cat curl sed uname mkdir systemctl basename"
    local cmd
    for cmd in $cmd_list; do
        type -P $cmd >/dev/null 2>&1 || fail "Unable to find '${cmd}' command. Ensure it is available in PATH '${PATH}' before continuing."
    done

    # detect package installation command
    cmd_list="yum dpkg"
    for cmd in $cmd_list; do
        pkg_cmd=$(type -P $cmd)
        if [[ $? -eq 0 ]]; then
            case "$(basename $pkg_cmd)" in
            (yum)
                pkg_ext=".rpm"
                pkg_args="localinstall -y"
                ;;
            (dpkg)
                pkg_ext=".deb"
                pkg_args="--install --force-confold"
                ;;
            esac
            break
        fi
    done

    [[ -n "${pkg_cmd:-}" ]] || fail "Unable to find a package install command ($cmd_list)"

    set -o errexit

    __parse_parameters "$@" 
    [[ -n "${ca_api_key:-}" ]] || fail "Circonus API key is *required*."
    [[ "select" == ${ca_broker_id,,} ]] && ca_broker_id=""

    log "Getting latest release version from repository"
    tag=$(__get_latest_release)
    ca_version=${tag#v}

    __get_os_sig
}

__make_circonus_dir() {
    local circ_dir="/opt/circonus"

    log "Creating Circonus base directory: ${circ_dir}"
    if [[ ! -d $circ_dir ]]; then
        \mkdir -p $circ_dir
        [[ $? -eq 0 ]] || fail "unable to create ${circ_dir}"
    fi

    log "Changing to ${circ_dir}"
    \cd $circ_dir
    [[ $? -eq 0 ]] || fail "unable to change to ${circ_dir}"
}

__get_ca_package() {
    local pkg="${pkg_file}${pkg_ext}"
    local url="${pkg_url}${pkg}"

    if [[ ! -f $pkg ]]; then
        log "Downloading agent package: ${url}"
        set +o errexit
        \curl -fsSLO "$url"
        curl_err=$?
        set -o errexit
        [[ $curl_err -eq 0 ]] || fail "unable to download ${url} ($curl_err)"
    fi

    [[ -f $pkg ]] || fail "unable to find ${pkg} in current dir"

    log "Installing: ${pkg_cmd} ${pkg_args} ${pkg}"
    $pkg_cmd $pkg_args $pkg
    [[ $? -eq 0 ]] || fail "installing ${pkg_cmd} ${pkg_args} ${pkg}"
}

__configure_agent() {
    log "Updating configuraiton: ${ca_conf_file}"

    if [[ ! -f $ca_conf_file ]]; then 
        # generate config if it doesn't exist
        # if it does, it may have been seeded by user, so don't change it
        ca_args="-C -r --api-key=${ca_api_key}"
        if [[ -n "${ca_api_app}" ]]; then
            ca_args="${cs_args} --api-app=${ca_api_app}"
        fi
        if [[ -n "${ca_broker_id}" ]]; then
            ca_args="${ca_args} --check-broker=${ca_broker_id}"
        fi
        cmd="/opt/circonus/agent/sbin/circonus-agentd"
        if [[ -x $cmd ]]; then
            log "Generating config ${ca_conf_file}"
            $cmd $ca_args --show-config=yaml > $ca_conf_file
            [[ $? -eq 0 ]] || fail "creating config ${ca_conf_file}"

            log "Restarting circonus-agent service"
            \systemctl restart circonus-agent
            [[ $? -eq 0 ]] || fail "systemctl restart circonus-agent failed"
        fi
    else
        echo "Existing configuration found, not updating..."
    fi
}

__get_os_sig() {
    local sig="${ca_os_sig:-}"
    local rh_release="/etc/redhat-release"
    local lsb_release="/etc/lsb-release"

    if [[ -z "${sig}" ]]; then
        if [[ -f $rh_release ]] ; then
            log "\tAttempt RedHat(variant) detection"
            release_rpm=$(/bin/rpm -qf $rh_release)
            el=$(expr "${release_rpm}" : '.*\(el[0-9]\)')
            if [[ -z "$el" ]]; then
                fail "Unsupported ${release_rpm}, unable to derive 'el' version"
            fi
            sig="${el}"
            log "\tDerived ${sig} from '${release_rpm}'"
        elif [[ -f $lsb_release ]]; then
            log "\tLSB found, using '${lsb_release}' for OS detection."
            source $lsb_release
            if [[ "ubuntu" == "${DISTRIB_ID,,}" ]]; then
                sig="${DISTRIB_ID,,}.${DISTRIB_RELEASE:-}"
                log "\tDerived ${sig} from ${lsb_release}"
            fi
        fi

        if [[ -z "$sig" ]]; then
            fail "Unable to detect rpm/deb installation os distro target"
        fi
    fi
    ca_os_sig="${sig}"
}

__get_latest_release() {
    if [[ -n "$ca_version" ]]; then
        echo $ca_version
    else 
        local url="https://api.github.com/repos/circonus-labs/circonus-agent/releases/latest"

        set +o errexit
        \curl -sS $url  | grep -Po '"tag_name": "\K.*?(?=")'
        curl_err=$?
        set -o errexit

        [[ $curl_err -eq 0 ]] || fail "unable to get latest release (${curl_err})"
    fi
}

ca_install() {

    __ca_init "$@"

    pkg_file="circonus-agent-${ca_version}-1.${ca_os_sig}_${pkg_arch}"
    pkg_url="https://github.com/circonus-labs/circonus-agent/releases/download/v${ca_version}/"

    log "Installing Circonus Agent v${ca_version} for ${ca_os_sig} ${pkg_arch}"

    ca_dir="/opt/circonus/agent"
    [[ -d $ca_dir ]] && fail "${ca_dir} previous installation directory found."

    __make_circonus_dir
    __get_ca_package
    __configure_agent

    echo
    echo
    pass "Circonus Agent v${ca_version} installed"
    echo
    log "Make any additional customization to configuration:"
    log "  ${ca_conf_file}"
    log "and restart agent for changes to take effect."
    echo
    echo
}

#
# no arguments are passed
#
if [[ $# -eq 0 ]]; then
    usage
    exit 0
fi
# short-circuit for help
if [[ "$*" == *--help* ]]; then
    usage
    exit 0
fi

#
# NOTE Ensure sufficient rights to do the install
#
(( UID != 0 )) && {
    printf "\n%b\n\n" "${RED}Must run as root[sudo] -- installing software requires certain permissions.${NORMAL}"
    exit 1
}

ca_install "$@"

# END