# v1.2.1

* fix: trim spaces from each user supplied check tag

# v1.2.0

* fix: use api ca file if specified for check api client
* add: generic hostTags method for check tags (applies to create and update)
* add: host tags to check (like cosi did)
* fix: only lower case category if not already encoded (affected receiver w/streamtagged metric names)
* upd: go1.15 manual tls VerifyConnection
* upd: depedencies
* fix: config file path sep to be os sensitive

# v1.1.0

* doc: add multi-agent details
* fix: linit align structs
* fix: lint for rand
* add: support tag merging for receiver
* upd: remove deprecated state mgmt
* add: force enterprise for multi-agent mode
* add: verify httptrap is enabled on broker for multi-agent mode
* add: SubmissionURL method for multi-agent
* upd: remove old enable new metrics support for managed checks
* add: VerifyConnection to tls config go1.15
* add: multi-agent submitter
* add: multi-agent options
* upd: default collectors if not set and not multi-agent mode
* add: multiple agent (single check) support - requires enterprise brokers
* doc: update defaults for options
* doc: add new flags
* doc: update to reflect no longer NAD drop in replacement
* upd: struct alignment
* add: enable maligned linter
* fix: binary names for containers
* add: openbsd x86_64 target
* upd: dependencies
* add: context to reverse primary broker instance check
* upd: refactor connection handling for reverse when broker closes connection due to simultaneous attempts for same check from multiple agents
* upd: explicit cases for prometheus metric types
* add: *WithContext to api methods
* add: golangci-lint action

# v1.0.14

* fix: use manual tls verify workaround for go1.15

# v1.0.13

* upd: circonus-agent-plugins

# v1.0.12

* fix: test for new url parse error format
* add: tests for bundle searching in multiple bundles found scenarios
* fix: return matched not found when no bundles created by agent after multiple bundles were found

# v1.0.11

* upd: if multiple checks found matching criteria (active,json:nad,target) and none match the agent, return result such that a check will be created (if create check is enabled) - note, this does present the possiblity of multiple checks being created if the notes are altered in such a way that the agent is not able to determine it created the check

# v1.0.10

* upd: remove rpm conflict with NAD

# v1.0.9

* add: `--check-update|-U` (check.update) force update of **ALL** configurable check bundle attributes:
  * config.url
  * target
  * display name
  * period
  * timeout
  * metric filters
  * check tags
  * broker cid if explicit (Meaning, agent will not select a new one for an existing check. It will only update if a broker id/cid is provided in the configuration)
  * NOTE: check-update takes precedence over check-update-metric-filters
* add: `--check-period` (check.period) default 60
* add: `--check-timeout` (check.timeout) default 10

# v1.0.8

* add: `--check-metric-filter-file` external json file with metric filters
* add: `etc/example_metric_filters.json` as an example of external metric filter file
* add: `--check-update-metric-filters` force updating the check bundle with configured metric filters at start

# v1.0.7

* upd: when multiple bundles returned from API, identify the one created by agent (vs. created by NAD/cosi)
* upd: pre-seed procfs/cpu for `cpu_used`

# v1.0.6

* add: `collector:cpu` - `num_cpu`, `processes`, `procs_runnable`, and `procs_blocked` for USE dashboard
* fix: `cpu_used` calculate average over collection interval (not aggregate)
* fix: increase max comm read timeouts to 6 (when waiting for command in reverse)

# v1.0.5

* fix: pin `x/sys` to fix `cannot use type []byte as type []int8` issue for freeebsd

# v1.0.4 _unreleased_

* upd: hide deprecated settings in config file (`--show-config`)
* upd: hide deprecated command line parameters
* upd: dependencies
* upd: replace deprecated state dir with new cache dir (release build & packaging)

# v1.0.3

* add: cache dir to RPM for circkpkg plugin

# v1.0.2

* fix: clustered broker selection, elide port from cn on identified owner

# v1.0.1

* add: `--statsd-addr`, `CA_STATSD_ADDR` to explicitly specify an address that statsd should listen to (e.g. `--statsd-addr=0.0.0.0` for docker containers, so the port can be properly exposed).
* fix: procfs.disk use `HOST_SYS` if provided

# v1.0.0

* add: nvidia gpu metrics builtin for windows platform

# v1.0.0-beta.9

* add: cluster mode statsd gauges as histogram capability (so each node is represented with _one_ sample)
* add: cluster mode statsd counters and sets as histograms with `statsd_type:count` tag
* add: cluster mode enable/disable builtins
* add: cluster mode configuration options
* add: zpool plugin
* add: include all service configurations in releases

# v1.0.0-beta.8

* add: EXPOSE to Dockerfile(s) for default listening ports

# v1.0.0-beta.7

* add: docker images
* add: linux_arm64 build
* upd: dependencies
* add: command line options and environment vars for builtin collector paths:
  * `--host-proc`, `HOST_PROC` = `/proc`
  * `--host-sys`, `HOST_SYS` = `/sys`
  * `--host-etc`, `HOST_ETC` = `/etc`
  * `--host-var`, `HOST_VAR` = `/var`
  * `--host-run`, `HOST_RUN` = `/run`

# v1.0.0-beta.6

* fix: pull broker CA cert from API for TLS config when refreshing check/broker

# v1.0.0-beta.5

* fix: regression test failure from diskstats update

# v1.0.0-beta.4

* add: support new metrics in kernel 4.18+ `diskstats` -- discards completed, discards merged, sectors discarded, discard ms
* add: `check_cid` and `check_uuid` to reverse log lines
* add: freebsd rc script

# v1.0.0-beta.3

* upd: support building packages for pre-releases
* upd: package builders
* upd: disable inclusion of `protocol_observer` binary in agent package builds

# v1.0.0-beta.2

* fix: gofmt io_latency plugin
* add: build plugins script
* add: build linting script
* add: `go mod tidy`, linting and plugin building before release
* add: `illumos` target to goreleaser builds.goos
* add: example metric filter using tags
* fix: lint, duplicate toml (one should be yaml)
* fix: lint, use `fmt.Println` vs `fmt.Printf` in test
* fix: lint, remove old `id`, replaced with streamtag `collector:promrecv`
* fix: generic builtins, skip NaN floats (causes json error)
* upd: dependencies
* add: smf manifest

# v1.0.0-beta.1

* fix: output all parsed plugin metrics with streamtags, include any tags from `_tags` attribute of emitted json
* fix: output tags `io_latency` in `_tags` attribute rather than in metric name (so they can be combined with agent tags to create stream tagged metric name)
* add: statsd tcp listner (optional, off by default)

# v1.0.0-alpha.5

* add: clustered broker support (initial)
* upd: do not exit when io_latency target dir already exists (artifact left when SIGKILL sent to child)
* upd: config option handling for procfs builtins
* fix: duplicate struct member causing blank procfs file for cpu
* add: `IgnoredMulti` metric for procfs/proto.{udp,udplite}
* add: `InType1` and `OutType` metrics for procfs/proto.icmp6
* upd: squelch "already running" error message for long running plugins (note, message is still emitted in debug)
* fix: io_latency output histograms as type `h`
* upd: module dependencies
* upd: go1.13

# v1.0.0-alpha.4

* testing release, not guaranteed to be feature complete
* fix: tests to include default stream tags
* fix: remove deprecated tests using metric states in builtins
* upd: module dependencies
* upd: go1.12.7

# v1.0.0-alpha.3

* testing release, not guaranteed to be feature complete
* upd: finish adding stream tags to wmi builtin collectors
* upd: remove obsolete code for deprecated settings from wmi builtin collectors

# v1.0.0-alpha.2

* testing release, not guaranteed to be feature complete
* add: integrate golangci check when PR opened
* more stringent linting
* upd: output errors for plugin parsing and exec
* fix: plugins, trim spaces from metric type (omnios plugin returns "L " for type)
* fix: handle deprecated procfs/diskstats and procfs/loadvg names (translate to procfs/disk and procfs/load)
* doc: update for new collector names
* add: finish wmi builtin collector(s)

# v1.0.0-alpha.1

* testing release, not guaranteed to be feature complete
* note that the wmi builtin collector(s) are still a WIP
* upd: **BREAKING CHANGE** agent v1+ will only support stream tags. Continue to use v0 releases to maintain continuity with existing metric names used in checks, visuals and alert rules.
