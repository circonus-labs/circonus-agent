# v0.18.0

* upd: remove Gopkg.* 
* upd: switch to github.com/circonus-labs/circonus-gometrics/v3
* fix: default metric_filters setting
* upd: short-circuit when using metric_filters to disable check management
* fix: remove placeholder metrics when creating a check with metric_filters
* upd: refactor to allow metric_filters to override manual check management
* upd: switch to github.com/circonus-labs/go-apiclient from github.com/circonus-labs/circonus-gometrics/api
* add: Check Metric Filter config option
* add: debug message before wait
* fix: typo in error message
* fix: missing cgm import after condense
* upd: reorg/condense
* upd: add consts for repeated strings
* upd: normalize proc fs path
* upd: cleanup error messages
* upd: refactor tomb -> errgroup and context
* fix: tweak for unattended fb11 build

# v0.17.2

* doc: add preview blurb to readme
* upd: explicitly skip "README.md" when scanning for plugins
* fix: fb11 use sudo for specific provision commands (needed for re-provisioning)
* add: Ubuntu 18.04 builder
* upd: allow building specific targets with vbuild.sh
* upd: bash login shell for freebsd builder

# v0.17.1

* add: dev package builder script (`vbuild.sh` uses Vagrant VMs to build development packages for testing)
* upd: default to blank statsd host prefix - send all to system check
* add: go.{mod,sum} to facilitate builds outside of GOPATH w/go1.11
* upd: switch agent/logwatch package names 64-bit -> x86_64
* upd: dependencies

# v0.17.0

* fix: version wedge when using snapshot releases for deb packages
* upd: allow 'snapshot' releases to be used in builds for dev/testing
* upd: release file names use x86_64, facilitate automated builds and testing
* upd: dependencies
* upd: refactor/condense api, prep for release

# v0.16.2

* upd: goreleaser, turn off draft
* upd: refactor rhel sysv restart
* fix: ubuntu start-stop-daemon add timeout to stops to allow conns to cleanup
* fix: ubuntu init.d/circonus-agent 0755
* upd: fix ubuntu service config installs
* upd: centos build vm to 7.4
* upd: disable logging for sysv init scripts
* upd: use `@@SBIN@@` in service configs and builder script
* upd: service script installs to account for alternate install_prefix
* upd: show file copies
* upd: builder (el6, el7, ubuntu14, ubuntu16, tgz)
* doc: service directory configs
* add: sysv init configs

# v0.16.1

* wip: package builder (incomplete)
* add: X-Circonus-Check-ID header to /write and /prom PUT|POST request responses
* add: X-Circonus-Check-Bundle-ID header to /write and /prom PUT|POST request responses
* fix: add pattern to wrapped error message for reverse url
* fix: use `git://` rather than `https://` urls for repos (older el build support)
* upd: use unique virtualbox names
* upd: release pkg name (macOS->darwin)
* upd: try different habitat origin for auto build
* doc: add details on getting prometheus in/out of agent
* upd: misc log message updates (level, etc.)

# v0.16.0

* fix: add error to reset command result so it is honored
* fix: remove deadlock in setReverseConfig
* upd: revert max reverse conn retries back to infinite (-1)
* upd: switch server request logging to debug level
* fix: handle enable new metrics when there's an error with the state directory. Disable new metric enabling and turn off check management, but allow check to be created.
* upd: export plugin/builtin stat times as strings for json

# v0.15.1

* fix: lock contention in check refresh
* upd: optimize lastMetrics (val->ptr)
* upd: additional debugging during run, flush, package
* upd: concurrent flushes (builtins, plugins, receiver, statsd, prom receiver)
* fix: var name typo
* upd: `lastMetrics` handling

# v0.15.0

* mrg: tracking upstream
* add: check bundle id to all reverse log messages
* upd: fatal when errors starting main listening servers (e.g. address already in use)
* upd: add more debug lines around refreshing check configuration
* fix: lock contention when refreshing check configuration for reverse AND enable new metrics is active
* add: `--reverse-max-conn-retry` command line option
* wip: gate max requests from broker before forced reset
* wip: refactor of inventory endpoint
* upd: cosi v2 prep

# v0.14.1

* upd: io latency plugin for linux - add additional signals to handler and cleanup in tracing start

# v0.14.0

* add: io latency plugin for linux

# v0.13.4

* upd: upstream deps (cgm)

# v0.13.3

* upd: upstream deps

# v0.13.2

* fix: linxu/procfs/diskstats collector. guard against malformed/blank lines
* doc: miscellaneous documentation updates

# v0.13.1

* fix: remove version line from `--show-config` output
* new: api package

# v0.13.0

* upd: refactor enable new metric logic to work better for new check use case
* upd: refactor reverse connection and command processing

# v0.12.0

* add: `-E` as a short option for `--check-enable-new-metrics`
* add: `--check-metric-state-dir` configuration option
* doc: stream tag support for JSON plugin output and JSON receiver
* add: stream tag support to parsers for JSON plugin output and JSON receiver /write/... endpoint
* doc: stream tag support to exec plugin output
* add: stream tag support to exec plugin output parser
* doc: stream tag support for StatsD receiver
* add: stream tag support to StatsD receiver metric parser
* upd: fix StatsD metric syntax for rates, require `|@`
* upd: optimize label handling in prometheus receiver/collector
* add: tags package, centralize handling of tag list to stream tag spec
* upd: allow underscore in plugin name with /run/... endpoint
* add: error message when both `--check-create` and `--check-id` options are specified
* upd: build constraint to go1.10

# v0.11.0

* upd: remove deprecated options/defaults
* doc: clarify comment in frame handling
* upd: honor RESETs from broker on reverse
* upd: use stream tag syntax by default for supported sources
* add: warn logging on duplicate metric names
* add: debug log summary of lines processed, metrics found, errors, and duplicates found while processing exec plugin output
* add: --debug-dump-metrics option to dump json sent to broker (option argument is a directory name where the user running circonus-agentd has write permissions. output file name format metrics_ccyymmdd_hhmmss.json)
* add: more tests to check package (state)
* add: more tests to check package (metrics)
* upd: switch from using map[string]interface{} to only using cgm.Metrics for metrics
* fix: issue with incrementing offset by bytes sent rather than payload length
* upd: check.New func signature in reverse tests
* upd: dependencies
* upd: remove httpgzip constraint
* add: tests for ParseListen
* doc: comment ParseListen

# v0.10.0

* fix: revert to check bundle updates vs check bundle metrics endpoint until histogram issue resolved
* add: state directory requirement for enable new metrics feature
* add: ability to automatically enable new metrics
* add: ability to have agent create a new check when not using reverse
* upd: reorganize check handling into dedicated package
* upd: :warning: **BREAKING** changes to configuration options *check* configuration options changed/reorganized
* add: example systemd service configuration to release tarball
* add: `--no-gzip` to force disable gzip'd responses
* add: `Transfer-Encoding: identity` to non-chunked responses
* add: fix chunked gzip responses, broker does not understand them

# v0.9.2

* upd: add circleci for integration testing
* upd: add linux arm release FOR TESTING (e.g. raspbian on pi3)
* upd: replace deprecated tr.CancelRequest w/context
* upd: defer context cancel() call
* fix: several test t.Fatalf calls
* upd: simplify (gofmt -s)
* fix: var shadow instances
* fix: typos in messages and comments
* doc: fix configuration link in main README.md
* doc: clarify configuration file quick start instructions in etc/README.md
* doc: fix typo (cirocnus) etc/README.md
* upd: dependencies
* upd: import path for httpgzip

# v0.9.1

* upd: constrain IDs provided in /run and /write URLs to [a-zA-Z0-9-]; ensure clean metric name prefixes
* fix: make plugins optional, do _not_ fatal error if no plugins are found

# v0.9.0

* add: initial mvp prometheus collector (pull prometheus text formatted metrics from an endpoint)
* add: initial mvp prometheus receiver (accept pushed prometheus text formatted metrics)

# v0.8.0

* fix: socket server test to fail correctly when on a linux vagrant mounted fs
* fix: various tests for cross platform differences
* fix: several tests to accommodate differences in os error messages across platforms
* add: bytes read/written to procfs.diskstats (eliminate need for nad:linux/disk.sh)
* fix: normalize procfs.cpu clockHZ
* fix: double backtick procfs.if tcp metrics
* upd: expose all procfs.vm meminfo metrics as raw names
* add: linux default collectors `['cpu', 'diskstats', 'if', 'loadavg', 'vm']`
* add: procfs.loadavg collect (nad:common/loadavg.elf)
* add: procfs.vm collector (nad:linux/vm.sh)
* add: procfs.if collector (nad:linux/if.sh)
* add: procfs.diskstats collector (nad:linux/diskstats.sh)
* add: procfs.cpu collector (nad:linux/cpu.sh)
* doc: `etc/README.md` and `plugins/README.md` to releases
* doc: reorganize documentation
* doc: add `plugins/README.md` with documentation about plugins
* doc: add `etc/README.md` with documentation specifically about configuring the Circonus agent and builtin collectors
* upd: vendor deps

# v0.7.0

* add: builtin collector framework
* new: configuration option `--collectors` (for platforms which have builtin collectors - windows only in this release)
* add: wmi builtin collectors for windows
    * available WMI collectors: cache, disk, interface, ip, memory, object, paging_file, processes, processor, tcp, udp
    * default WMI collectors enabled `['cache', 'disk', 'ip', 'interface', 'memory', 'object', 'paging_file' 'processor', 'tcp', 'udp']`
* new: collectors take precedence over plugins (e.g. collector named `cpu` would prevent plugin named `cpu` from running)
* upd: plugin directory is now optional - valid use case to run w/o plugins - e.g. only builtins, statsd, receiver or a combination of the three
* upd: select _fastest_ broker rather than picking randomly from list of _all_ available brokers. If multiple brokers are equally fast, fallback to picking randomly, from the list of _fastest_ brokers.
* fix: use value of `--reverse-target` (if specified) in `--reverse-create-check-title` (if not specified)

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
