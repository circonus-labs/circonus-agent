# Circonus Agent

>NOTE: This is an "in development" project. As such, there are a few things to be aware of at this time...
>
> Caveats:
> * No target specific packages. (e.g. rpm|deb|pkg)
> * No service configurations provided. (e.g. systemd, upstart, init, svc)
> * Native plugins (.js) do not work. Unless modified to run `node` independently and follow [plugin output guidelines](#output).
>
> The code is changing frequently at this point. Before reporting an issue, please ensure the [latest release](https://github.com/circonus-labs/circonus-agent/releases/latest) is being used.

# Quick Start

> Installing on a system which has already had [cosi](https://github.com/circonus-labs/circonus-one-step-install) install and configure NAD.

1. `mkdir -p /opt/circonus/agent/{sbin,etc}`
1. Download [latest release](https://github.com/circonus-labs/circonus-agent/releases/latest) from GitHub repository (or [build manually](#manual-build))
1. If downloaded, extract archive into `/opt/circonus/agent`
1. Stop NAD (e.g. `systemctl stop nad`)
1. Create a [config](#config) or use command line parameters
1. Run `sbin/circonus-agentd`

Example, minimal, configuration using existing cosi install `/opt/circonus/agent/etc/circonus-agent.toml`:

```toml
# set the plugin directory to NAD's
plugin-dir = "/opt/circonus/nad/etc/node-agent.d"

[reverse]
enabled = true
cid = "cosi" # use cosi system check bundle

[api]
key = "cosi" # use cosi api configuration

#debug = true
```

# Options

```
$ /opt/circonus/agent/sbin/circonus-agentd -h
```

```
Flags:
      --api-app string                       [ENV: CA_API_APP] Circonus API Token app (default "circonus-agent")
      --api-ca-file string                   [ENV: CA_API_CA_FILE] Circonus API CA certificate file
      --api-key string                       [ENV: CA_API_KEY] Circonus API Token key
      --api-url string                       [ENV: CA_API_URL] Circonus API URL (default "https://api.circonus.com/v2/")
  -c, --config string                        config file (default is /opt/circonus/agent/etc/circonus-agent.(json|toml|yaml)
  -d, --debug                                [ENV: CA_DEBUG] Enable debug messages
      --debug-cgm                            [ENV: CA_DEBUG_CGM] Enable CGM & API debug messages
  -h, --help                                 help for circonus-agent
  -l, --listen string                        [ENV: CA_LISTEN] Listen address and port [[IP]:[PORT]] (default ":2609")
      --log-level string                     [ENV: CA_LOG_LEVEL] Log level [(panic|fatal|error|warn|info|debug|disabled)] (default "info")
      --log-pretty                           [ENV: CA_LOG_PRETTY] Output formatted/colored log lines
      --no-statsd                            [ENV: CA_NO_STATSD] Disable StatsD listener
  -p, --plugin-dir string                    [ENV: CA_PLUGIN_DIR] Plugin directory (default "/opt/circonus/agent/plugins")
  -r, --reverse                              [ENV: CA_REVERSE] Enable reverse connection
      --reverse-broker-ca-file string        [ENV: CA_REVERSE_BROKER_CA_FILE] Broker CA certificate file
      --reverse-cid string                   [ENV: CA_REVERSE_CID] Check Bundle ID for reverse connection
      --reverse-create-check                 [ENV: CA_REVERSE_CREATE_CHECK] Create check bundle for reverse if one cannot be found
      --reverse-create-check-broker string   [ENV: CA_REVERSE_CREATE_CHECK_BROKER] ID of Broker to use or 'select' for random selection of valid broker, if creating a check bundle (default "select")
      --reverse-create-check-tags string     [ENV: CA_REVERSE_CREATE_CHECK_TAGS] Tags [comma separated list] to use, if creating a check bundle
      --reverse-create-check-title string    [ENV: CA_REVERSE_CREATE_CHECK_TITLE] Title [display name] to use, if creating a check bundle (default "<reverse-target> /agent")
      --reverse-target string                [ENV: CA_REVERSE_TARGET] Target host (default hostname)
      --show-config                          Show config and exit
      --ssl-cert-file string                 [ENV: CA_SSL_CERT_FILE] SSL Certificate file (PEM cert and CAs concatenated together) (default "/opt/circonus/agent/etc/circonus-agent.pem")
      --ssl-key-file string                  [ENV: CA_SSL_KEY_FILE] SSL Key file (default "/opt/circonus/agent/etc/circonus-agent.key")
      --ssl-listen string                    [ENV: CA_SSL_LISTEN] SSL listen address and port [IP]:[PORT] - setting enables SSL
      --ssl-verify                           [ENV: CA_SSL_VERIFY] Enable SSL verification (default true)
      --statsd-group-cid string              [ENV: CA_STATSD_GROUP_CID] StatsD group check bundle ID
      --statsd-group-counters string         [ENV: CA_STATSD_GROUP_COUNTERS] StatsD group metric counter handling (average|sum) (default "sum")
      --statsd-group-gauges string           [ENV: CA_STATSD_GROUP_GAUGES] StatsD group gauge operator (default "average")
      --statsd-group-prefix string           [ENV: CA_STATSD_GROUP_PREFIX] StatsD group metric prefix (default "group.")
      --statsd-group-sets string             [ENV: CA_STATSD_GROPUP_SETS] StatsD group set operator (default "sum")
      --statsd-host-cateogry string          [ENV: CA_STATSD_HOST_CATEGORY] StatsD host metric category (default "statsd")
      --statsd-host-prefix string            [ENV: CA_STATSD_HOST_PREFIX] StatsD host metric prefix (default "host.")
      --statsd-port string                   [ENV: CA_STATSD_PORT] StatsD port (default "8125")
  -V, --version                              Show version and exit
 ```

# Config

YAML
```yaml
---
debug: false
debug_cgm: false
listen: ":2609"
plugin-dir: "/opt/circonus/agent/plugins"

api:
  app: circonus-agent
  ca_file: ''
  key: ''
  url: https://api.circonus.com/v2/

log:
  level: info
  pretty: false

reverse:
  enabled: false
  broker_ca_file: ''
  check_bundle_id: ''
  check_target: localhost
  create_check: false
  check:
    broker: select
    tags: ''
    title: localhost /agent

ssl:
  cert_file: "/opt/circonus/agent/etc/circonus-agent.pem"
  key_file: "/opt/circonus/agent/etc/circonus-agent.key"
  listen: ''
  verify: true

statsd:
  disabled: false
  port: '8125'
  group:
    check_bundle_id: ''
    counters: sum
    gauges: average
    metric_prefix: group.
    sets: sum
  host:
    category: statsd
    metric_prefix: host.

```

TOML
```toml
debug = false
debug_cgm = false
listen = ":2609"
plugin-dir = "/opt/circonus/agent/plugins"

[api]
app = "circonus-agent"
ca_file = ""
key = ""
url = "https://api.circonus.com/v2/"

[log]
level = "info"
pretty = false

[reverse]
enabled = false
broker_ca_file = ""
check_bundle_id = ""
check_target = "localhost"
create_check = false
[reverse.check]
broker = "select"
tags = ""
title = "localhost /agent"

[ssl]
cert_file = "/opt/circonus/agent/etc/circonus-agent.pem"
key_file = "/opt/circonus/agent/etc/circonus-agent.key"
listen = ""
verify = true

[statsd]
disabled = false
port = '8125'

[statsd.group]
check_bundle_id = ""
counters = "sum"
gauges = "average"
metric_prefix = "group."
sets = "sum"

[statsd.host]
category = "statsd"
metric_prefix = "host."
```

JSON
```json
{
   "api": {
     "app": "circonus-agent",
     "ca_file": "",
     "key": "",
     "url": "https://api.circonus.com/v2/"
   },
   "debug": false,
   "debug_cgm": false,
   "listen": ":2609",
   "log": {
     "level": "info",
     "pretty": false
   },
   "plugin-dir": "/opt/circonus/agent/plugins",
   "reverse": {
     "broker_ca_file": "",
     "check": {
       "broker": "select",
       "tags": "",
       "title": "localhost /agent"
     },
     "check_bundle_id": "",
     "check_target": "localhost",
     "create_check": false,
     "enabled": false
   },
   "ssl": {
     "cert_file": "/opt/circonus/agent/etc/circonus-agent.pem",
     "key_file": "/opt/circonus/agent/etc/circonus-agent.key",
     "listen": "",
     "verify": true
   },
   "statsd": {
     "disabled": false,
     "group": {
       "check_bundle_id": "",
       "counters": "sum",
       "gauges": "average",
       "metric_prefix": "group.",
       "sets": "sum"
     },
     "host": {
       "category": "statsd",
       "metric_prefix": "host."
     },
     "port": "8125"
   }
 }
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

## Running plugin environment

When plugins are executed, the _current working directory_ will be set to the `--plugin-dir`, for relative path references to find configs or data files. Scripts may safely reference `$PWD`. See `plugin_test/write_test/wtest1.sh` for example. In `plugin_test`, run `ln -s write_test/wtest1.sh`, start the agent (e.g. `go run main.go -p plugin_test`), then `curl localhost:2609/` to see it in action.

## Plugin Output

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


# Manual build

1. Clone repo `git clone https://github.com/circonus-labs/circonus-agent.git`
1. Dependencies, run `dep ensure` (requires [dep](https://github.com/golang/dep) utility)
1. Build `go build -o circonus-agentd`
1. Install `cp circonus-agentd /opt/circonus/agent/sbin`


[![codecov](https://codecov.io/gh/maier/circonus-agent/branch/master/graph/badge.svg)](https://codecov.io/gh/maier/circonus-agent)
