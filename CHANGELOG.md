# v0.7.0

* Windows builtin collectors (using WMI)
* New configuration option `--collectors` (for platforms that support builtin collectors - currently windows only)
* Available WMI collectors: cache, disk, interface, ip, memory, object, paging_file, processes, processor, tcp, udp
* Default WMI collectors enabled `['cache', 'disk', 'ip', 'interface', 'memory', 'object', 'paging_file' 'processor', 'tcp', 'udp']`
* Builtin collectors take precedence over plugins (e.g. a builtin named `cpu` would prevent a plugin named `cpu` from being run)
* Plugin directory is now optional (valid use case to run w/o plugins - e.g. only builtins or statsd or receiver or a combination of the three)
* Attempt to select _fastest_ broker rather than picking randomly from list of _all_ available brokers. If multiple brokers are equally fast, fallback to picking randomly from list of _fastest_ brokers.

# v0.6.0

* exit agent is issue creating/starting any server (http, ssl, sock)
* config file setting renamed `plugin-dir` -> `plugin_dir` to match other settings
* add unix socket listener support (for `/write` endpoint only)
    * command line option, one or more, `-L </path/to/socket_file>` or `--listen-socket=</path/to/socket_file>`
    * config file `listen_socket` (array of strings)
    * handle encoded histograms (e.g. cgm sending to agent `/write` endpoint)
* add ttl capability to plugins; parsed from plugin name (e.g. `test_ttl5m.sh` run once every five minutes) valid units `ms`, `s`, `m`, `h`.
* allow multiple listen ip:port specs to be used (e.g. `-l 127.0.0.1:2609 -l 192.168.1.2:2630 ...`)
* migrate configuration validation to (server|statsd|plugins).New functions
* docs: add new `--plugin-ttl-units` default `s`[econds]
* docs: add new `-L, --listen-socket`
* docs: update `--listen` allow more than one
* docs: update `--show-config` now requires a format to output (`json`|`toml`|`yaml`)

Socket example:

```sh
# start agent with the additional setting
$ /opt/circonus/agent/sbin/circonus-agentd ... --listen-socket=/tmp/test.sock

$ curl --unix-socket /tmp/test.sock \
    -H 'Content-Type: application/json' \
    -d '{"test":{"_type":"i","_value":1}}' \
    http:/circonus-agent/write/socktest

# resulting metric: socktest`test numeric 1
```

Example configuring CGM to use an agent socket named `/tmp/test.sock`:

```go
cmc := &cgm.Config{}

cmc.CheckManager.Check.SubmissionURL = "http+unix:///tmp/test.sock/write/prefix_id"
// prefix_id will be the first part of the metric names

cmc.CheckManager.API.TokenKey = ""
// disable check management and interactions with API

client, err := cgm.New(cmc)
if err != nil {
    panic(err)
}

client.Increment("metric_name")

// resulting in metric: prefix_id`metric_name numeric 1
```


# v0.5.0

* standardize on cgm.Metric(s) structs for all metrics
* strict parsing of JSON sent to receiver `/write` endpoint
* add `/prom` endpoint (candidate poc)
* improve handling of invalid sized reads
* group plugin stats
* update cgm version
* common appstats package

# v0.4.0

* switch --show-config to take a format argument (json|toml|yaml)
* add more test coverage
* reorganize README
* update dependencies

# v0.3.0

* add running config settings to app stats
* add env vars for options to help output
* switch env var prefix from `NAD_` to `CA_`

# v0.2.0

* add ability to create a [reverse] check - if a check bundle id is not provided for reverse, the agent will search for a suitable check bundle for the host. Previously, if a check bundle could not be found the agent would exit. Now, when `--reverse-create-check` is supplied, the agent has the ability to create a check, rather than exit.
* expose basic app stats endpoint /stats

# v0.1.2

* fix statsd packet channel (broken in v0.1.1)
* update readme with current instructions

# v0.1.1

* merge structs
* eliminate race condition

# v0.1.0

* add freebsd and solaris builds for testing
* add more test coverage throughout
* switch to tomb instead of contexts
* refactor code throughout
* add build constraints to control target specific signal handling in agent package
* fix race condition w/inventory handler
* reset connection attempts after successful send/receive (catch connection drops)
* randomize connection retry attempt delays (not all agents retrying on same schedule)

# v0.0.3

* integrate context
* cleaner shutdown handling

# v0.0.2

* move `circonus-agentd` binary to `sbin/circonus-agentd`
* refactor (plugins, server, reverse, statsd)
* add agent package

# v0.0.1

* Initial development working release
