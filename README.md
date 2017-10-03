# Circonus Agent

>NOTE: This is an "in development" project. As such, there are a few things to be aware of at this time...
>
> Caveats:
> * No target specific packages. (e.g. rpm|deb|pkg)
> * No service configurations provided. (e.g. systemd, upstart, init, svc)
> * Native plugins (.js) do not work. Unless modified to run `node` independently and follow [plugin output guidelines](#output).

## development working release

Download latest release from repo [releases](https://github.com/circonus-labs/circonus-agent/releases).

Example of installing into an existing COSI registered linux system.

```sh
cd /opt/circonus
mkdir -p agent/{sbin,etc}
cd agent
ln -s /opt/circonus/nad/etc/node-agent.d plugins
curl "https://github.com/circonus-labs/circonus-agent/releases/download/v0.1.2/circonus-agent_0.1.2_linux_64-bit.tar.gz" -o circonus-agent.tgz
tar zxf circonus-agent.tgz
```
To leverage the existing COSI/NAD installation, create a configuration file `/opt/circonus/agent/etc/circonus-agent.toml` (or use the corresponding command line options.)

```toml
[reverse]
enabled = true
cid = "cosi"

[api]
key = "cosi"
```

Ensure that NAD is not currently running (e.g. `systemctl stop nad`) and start circonus-agent `sbin/circonus-agentd`.

## development testing notes

> NOTE: See `Vagrantfile` for an example which bootstraps a centos7 vm.

1. Install go (on linux for example)
    1. `curl "https://storage.googleapis.com/golang/go1.9.linux-amd64.tar.gz" -O`
    1. `sudo tar -C /usr/local -xzf go1.9.linux-amd64.tar.gz`
    1. `echo 'export PATH="$PATH:/usr/local/go/bin"' > /etc/profile.d/go.sh` (root)
1. Setup go env & clone repo
    1. `mkdir -p ~/godev/src/github.com/circonus-labs`
    1. Set `GOPATH="${HOME}/godev"` in `.bashrc`
    1. Logout/login, verify `go version && env | grep GOPATH`
1. Install dep (go package dependency manager)
    1. `go get -u github.com/golang/dep/cmd/dep`
1. Clone repo and run dep
    1. `cd $GOPATH/src/github.com/circonus-labs`
    1. `git clone https://github.com/circonus-labs/circonus-agent.git`
    1. `cd circonus-agent && dep ensure`
1. Build
    1. `go build`
1. Install
    1. `mkdir -p /opt/circonus/agent/sbin`
    1. `cp circonus-agent /opt/circonus/agent/sbin/circonus-agentd`
1. Run
    1. `/opt/circonus/agent/sbin/circonus-agentd -h` for help
    1. example - on a system where cosi has *already* been run
       1. stop nad, if it is running
       1. run: `/opt/circonus/agent/sbin/circonus-agentd -p /opt/circonus/nad/etc/node-agent.d -r -cid cosi -api-key cosi --log-pretty`
        * `-p` use the existing nad plugins
        * `--api-key cosi` load api credentials from cosi installation
        * `--reverse-cid cosi` load check information from cosi installation
        * `-r` establish a reverse connection
        * `--log-prety` print formatted logging output to the terminal

# Options

```
$ /opt/circonus/agent/sbin/circonus-agentd -h
```

# Plugins

* Go in the `--plugin-dir`.
* Must be regular files or symlinks.
* Must be executable (e.g. `0755`)
* Files are expected to be named matching a pattern of: `<base_name>.<ext>` (e.g. `foo.sh`)
* Directories are ignored.
* Configuration files are ignored.
    * Configuration files are defined as files with extensions of `.json` or `.conf`
    * A `.json` file is assumed to be a configuration for a plugin with the same `base_name` (e.g. `foo.json` is a configuration for `foo.sh`, `foo.elf`, etc.)
        * JSON config files are loaded and arguments defined are passed to the plugin instance(s).
        * The format for JSON config files is: `{"instance_id": ["arg1", "arg2", ...], ...}`.
        * One instance of the plugin will be run for each distinct `instance_id` found in the JSON.
        * The format of the resulting metric names would be: **plugin\`instance_id\`metric_name**
    * A `.conf` file is assumed to be a shell configuration file which is loaded by the plugin itself.
* All other directory entries are ignored.

## Output

Output from plugins is expected on `stdout` either tab-delimited or json.

### Tab delimited

`metric_name<TAB>metric_type<TAB>metric_value`

### JSON

```json
{
    "metric_name": {
        "_type": "metric_type",
        "_value": "metric_value"
    }
}
```

### Metric types

| Type | Description             |
| ---- | ----------------------- |
| `i`  | signed 32-bit integer   |
| `I`  | unsigned 32-bit integer |
| `l`  | signed 64-bit integer   |
| `L`  | unsigned 64-bit integer |
| `n`  | double/float            |
| `s`  | string/text             |

## Running

When plugins are executed, the _current working directory_ will be set to the `--plugin-dir`, for relative path references to find configs or data files. Scripts may safely reference `$PWD`. See `plugin_test/write_test/wtest1.sh` for example. In `plugin_test`, run `ln -s write_test/wtest1.sh`, start the agent (e.g. `go run main.go -p plugin_test`), then `curl localhost:2609/` to see it in action.

[![codecov](https://codecov.io/gh/maier/circonus-agent/branch/master/graph/badge.svg)](https://codecov.io/gh/maier/circonus-agent)
