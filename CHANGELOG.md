# v1.0.0-beta.9

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
