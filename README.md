# Circonus Agent

>NOTE: This is an "in development" project. As such, there are a few things to be aware of at this time...
>
> Caveats:
> * The code is *changing frequently* - please ensure the [latest release](../../releases/latest) is being used
> * No target specific packages. (e.g. rpm|deb|pkg)
> * No service configurations provided. (e.g. systemd, upstart, init, svc)
> * Native plugins (.js) do not work. Unless modified to run `node` independently and follow [plugin output guidelines](#output)

# Features

1. Builtin metric [collectors](#builtin-collectors)
1. [Plugin](#plugins) architecture for local metric collection
1. Local HTTP [Receiver](#receiver) for POST/PUT metric collection
1. Local [StatsD](#statsd) listener for application metrics



# Quick Start

> Installing on a system which has already had [cosi](https://github.com/circonus-labs/circonus-one-step-install) install and configure NAD.

1. `mkdir -p /opt/circonus/agent/{sbin,etc}`
1. Download [latest release](../../releases/latest) from repository (or [build manually](#manual-build))
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
      --collectors stringSlice               [ENV: CA_COLLECTORS] List of builtin collectors to enable
  -c, --config string                        config file (default is /opt/circonus/agent/etc/circonus-agent.(json|toml|yaml)
  -d, --debug                                [ENV: CA_DEBUG] Enable debug messages
      --debug-cgm                            [ENV: CA_DEBUG_CGM] Enable CGM & API debug messages
  -h, --help                                 help for circonus-agent
  -l, --listen stringSlice                   [ENV: CA_LISTEN] Listen spec e.g. :2609, [::1], [::1]:2609, 127.0.0.1, 127.0.0.1:2609, foo.bar.baz, foo.bar.baz:2609 (default ":2609")
  -L, --listen-socket stringSlice            [ENV: CA_LISTEN_SOCKET] Unix socket to create
      --log-level string                     [ENV: CA_LOG_LEVEL] Log level [(panic|fatal|error|warn|info|debug|disabled)] (default "info")
      --log-pretty                           [ENV: CA_LOG_PRETTY] Output formatted/colored log lines [ignored on windows]
      --no-statsd                            [ENV: CA_NO_STATSD] Disable StatsD listener
  -p, --plugin-dir string                    [ENV: CA_PLUGIN_DIR] Plugin directory (default "/opt/circonus/agent/plugins")
      --plugin-ttl-units string              [ENV: CA_PLUGIN_TTL_UNITS] Default plugin TTL units (default "s")
  -r, --reverse                              [ENV: CA_REVERSE] Enable reverse connection
      --reverse-broker-ca-file string        [ENV: CA_REVERSE_BROKER_CA_FILE] Broker CA certificate file
      --reverse-cid string                   [ENV: CA_REVERSE_CID] Check Bundle ID for reverse connection
      --reverse-create-check                 [ENV: CA_REVERSE_CREATE_CHECK] Create check bundle for reverse if one cannot be found
      --reverse-create-check-broker string   [ENV: CA_REVERSE_CREATE_CHECK_BROKER] ID of Broker to use or 'select' for random selection of valid broker, if creating a check bundle (default "select")
      --reverse-create-check-tags string     [ENV: CA_REVERSE_CREATE_CHECK_TAGS] Tags [comma separated list] to use, if creating a check bundle
      --reverse-create-check-title string    [ENV: CA_REVERSE_CREATE_CHECK_TITLE] Title [display name] to use, if creating a check bundle (default "<reverse-target> /agent")
      --reverse-target string                [ENV: CA_REVERSE_TARGET] Target host (default <hostname>)
      --show-config string                   Show config (json|toml|yaml) and exit
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



# Configuration

The Circonus agent can be configured via the command line, environment variables, and/or a configuration file. For details on using configuration files, see the configuration section of [etc/README.md](etc/README.md#main-configuration)



# Plugins

For documentation on plugins please refer to [plugins/README.md](plugins/README.md).



# Receiver

The Circonus agent provides a special handler for the endpoint `/write` which will accept HTTP POST and HTTP PUT requests containing structured JSON.

The structure of the JSON expected by the receiver is the same as the JSON format accepted from plugins. See the [JSON](plugins/README.md#json) section of the [plugin documentation](plugins/README.md) for details on the structure.

The URL syntax for sending metrics is `/write/ID` where `ID` is a prefix for all of the metrics being sent in the request.

For example:

HTTP POST `http://127.0.0.1:2609/write/test` with a payload of:

```json
{
    "t1": {
        "_type": "i",
        "_value": 32
    },
    "t2": {
        "_type": "s",
        "_value": "foo"
    }
}
```

Would result in metrics in the Circonus UI of:

```
test`t1 numeric 32
test`t2 text "foo"
```



# StatsD

The Circonus  agent provides a StatsD listener by default (disable: `--no-statsd`, configure port: `--statsd-port`). It accepts the basic [StatsD metric types](https://github.com/etsy/statsd/blob/master/docs/metric_types.md#statsd-metric-types) as well as, Circonus specific metric types `h` and `t`.

| Type | Note                            |
| ---- | ------------------------------- |
| `c`  | Counter                         |
| `g`  | Gauge                           |
| `h`  | Histogram - Circonus specific   |
| `ms` | Timing - treated as a Histogram |
| `s`  | Sets - treated as a Counter     |
| `t`  | Text - Circonus specific        |

>NOTE: the derivative metrics automatically generated with some StatsD types are not created by Circonus, as the data is already available within the Circonus UI.



# Builtin collectors

The circonus-agent has builtin collectors offering a higher level of efficiency over executing plugins. The circonus-agent `--collectors` command line option controls which collectors are enabled. Builtin collectors take precedence over plugins - if a builtin collector exists with the same ID as a plugin, the plugin will not be activated. Configuration files for builtins are located in the circonus-agent `etc` directory (e.g. `/opt/circonus/agent/etc` or `C:\circonus-agent\etc`).

Configuration:

* Command line `--collectors` (space delimited list)
* Environment `CA_COLLECTORS` (space delimited list)
* Config file `collectors` (array of strings)

* Windows default WMI collectors: `['cache', 'disk', 'ip', 'interface', 'memory', 'object', 'paging_file' 'processor', 'tcp', 'udp']`
* Linux default ProcFS collectors: `['cpu']`
* Common `prometheus` (disabled if no configuration file exists)

For complete list of collectors and details on collector specific configuration see [etc/README.md](etc/README.md#collector-configurations).


# Manual build

1. Clone repo `git clone https://github.com/circonus-labs/circonus-agent.git`
1. Dependencies, run `dep ensure` (requires [dep](https://github.com/golang/dep) utility)
1. Build `go build -o circonus-agentd`
1. Install `cp circonus-agentd /opt/circonus/agent/sbin`
