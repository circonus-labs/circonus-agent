# Circonus Agent

>NOTE: This is an "in development" project. As such, there are a few things to be aware of at this time...
>
> Caveats:
> * No target specific packages
> * No service configurations provided
> * Native plugins (.js) do not work

## v0.0.1 development working release

Download from repo [releases](https://github.com/circonus-labs/circonus-agent/releases).

Example of installing into an existing COSI registered linux system.

```sh
cd /opt/circonus
mkdir -p circonus-agent/{sbin,etc}
cd circonus-agent
ln -s /opt/circonus/nad/etc/node-agent.d plugins
curl "https://github.com/circonus-labs/circonus-agent/releases/download/v0.1.0/circonus-agent_0.0.1_linux_64-bit.tar.gz" -o circonus-agent.tgz
tar zxf circonus-agent.tgz
```
To leverage the existing COSI/NAD installation, create a configuration file `/opt/circonus/circonus-agent/etc/circonus-agent.toml` (or use the corresponding command line options.)

```toml
[reverse]
enabled = true
cid = "cosi"

[api]
key = "cosi"
```

Ensure that NAD is not currently running (e.g. `systemctl stop nad`) and start circonus-agent `sbin/circonus-agent`.

## development testing notes 2017-09-06

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
    1. `git clone https://github.com/maier/circonus-agent.git`
    1. `cd circonus-agent && dep ensure`
1. Build
    1. `go build`
1. Run
    1. `./circonus-agent -h` for help
    1. example - if cosi had already been run, stop nad first, then: `./circonus-agent -p /opt/circonus/nad/etc/node-agent.d -r -cid cosi -api-key cosi --log-pretty` it _should_ start, load the api credentials and check information from the existing cosi config for reverse mode and use the existing nad plugins.

```
$ /opt/circonus/circonus-agent/sbin/circonus-agent -h
The Circonus host agent daemon provides a simple mechanism
to expose systems and application metrics to Circonus.
It inventories all executable programs in its plugin directory
and executes them upon external request, returning results
in JSON format.

Usage:
  circonus-agent [flags]

Flags:
      --api-app string                  Circonus API Token app (default "circonus-agent")
      --api-ca-file string              Circonus API CA certificate file
      --api-key string                  Circonus API Token key
      --api-url string                  Circonus API URL (default "https://api.circonus.com/v2/")
      --config string                   config file (default is /opt/circonus/circonus-agent/etc/circonus-agent.(json|toml|yaml)
  -d, --debug                           Enable debug messages
      --debug-cgm                       Enable CGM API debug messages
  -h, --help                            help for circonus-agent
  -l, --listen string                   Listen address and port [[IP]:[PORT]](default ":2609")
      --log-level string                Log level [(panic|fatal|error|warn|info|debug|disabled)] (default "info")
      --log-pretty                      Output formatted/colored log lines
      --no-statsd                       Disable StatsD listener
  -p, --plugin-dir string               Plugin directory (default "/opt/circonus/circonus-agent/plugins")
  -r, --reverse                         Enable reverse connection
      --reverse-broker-ca-file string   Broker CA certificate file
      --reverse-cid string              Check Bundle ID for reverse connection
      --reverse-target string           Target host (default "centos7")
      --show-config                     Show config and exit
      --ssl-cert-file string            SSL Certificate file (PEM cert and CAs concatenated together) (default "/opt/circonus/circonus-agent/etc/circonus-agent.pem")
      --ssl-key-file string             SSL Key file (default "/opt/circonus/circonus-agent/etc/circonus-agent.key")
      --ssl-listen string               SSL listen address and port [IP]:[PORT] - setting enables SSL
      --ssl-verify                      Enable SSL verification (default true)
      --statsd-group-cid string         StatsD group check bundle ID
      --statsd-group-counters string    StatsD group metric counter handling (average|sum) (default "sum")
      --statsd-group-gauges string      StatsD group gauge operator (default "average")
      --statsd-group-prefix string      StatsD group metric prefix (default "group.")
      --statsd-group-sets string        StatsD group set operator (default "sum")
      --statsd-host-cateogry string     StatsD host metric category (default "statsd")
      --statsd-host-prefix string       StatsD host metric prefix (default "host.")
      --statsd-port string              StatsD port (default "8125")
  -V, --version                         Show version and exit
```


## Plugins

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

### Output

Output from plugins is expected on `stdout` either tab-delimited or json.

#### Tab delimited

`metric_name<TAB>metric_type<TAB>metric_value`

#### JSON

```json
{
    "metric_name": {
        "_type": "metric_type",
        "_value": "metric_value"
    }
}
```

#### Metric types

| Type | Description             |
| ---- | ----------------------- |
| `i`  | signed 32-bit integer   |
| `I`  | unsigned 32-bit integer |
| `l`  | signed 64-bit integer   |
| `L`  | unsigned 64-bit integer |
| `n`  | double/float            |
| `s`  | string/text             |

### Running

When plugins are executed, the _current working directory_ will be set to the `--plugin-dir`, if needed for relative path references to find configs or data files. Scripts may safely reference `$PWD`. See `plugin_test/write_test/wtest1.sh` for example. In `plugin_test`, run `ln -s write_test/wtest1.sh`, start the agent (e.g. `go run main.go -p plugin_test`), then `curl localhost:2609/` to see it in action.
