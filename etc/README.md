# Circonus Agent

The `etc` directory is used for the main configuration file for the Circonus agent as well as, the optional, configuration files for builtin collectors.

# Main configuration

File name: `circonus-agent.(json|toml|yaml)`

An example configuration, with default values, can be retrieved using the `--show-config=(json|toml|yaml)`.

## Configuration file quick start

Run one of the following and edit the resulting configuration file:

```
sbin/cirocnus-agentd --show-config=json > etc/circonus-agent.json
sbin/cirocnus-agentd --show-config=toml > etc/circonus-agent.toml
sbin/cirocnus-agentd --show-config=yaml > etc/circonus-agent.yaml
```

or, on Windows:

```
sbin\cirocnus-agentd.exe --show-config=json > etc\circonus-agent.json
sbin\cirocnus-agentd.exe --show-config=toml > etc\circonus-agent.toml
sbin\cirocnus-agentd.exe --show-config=yaml > etc\circonus-agent.yaml
```

## Collector configurations

The same three formats (json, toml, yaml) are supported for collector configurations. The name of the collector configuration file is the name used to enable the collector in the `--collectors` command line argument, `collectors` item in the main configuration file, or `CA_COLLECTORS` environment variable.

All collectors have a basic set of configuration options:

| Option                   | Type             | Default            | Description |
| ------------------------ | ---------------- | ------------------ | ----------- |
| `id`                     | string           | name of collector  | ID/Name of the collector (used as prefix for metrics). |
| `metrics_enabled`        | array of strings | empty              | list of metrics which are enabled (to be collected) |
| `metrics_disabled`       | array of strings | empty              | list of metrics which are disabled (should NOT be collected) |
| `metrics_default_status` | string           | `enabled`          | how a metric NOT in the enabled/disabled lists should be handled ("enabled" or "disabled") |
| `metric_name_regex`      | string           | `[^a-zA-Z0-9.-_:]` | regular expression of valid characters for the metric names |
| `metric_name_char`       | string           | `_`                | used for replacing invalid characters in a metric name (those not matching `metric_name_regex`) |
| `run_ttl`                | string           | empty              | indicating collector will run no more frequently than TTL (e.g. "10s", "5m", etc. - for expensive collectors) |

Additionally, each collector may have more configuration options specific to _what_ is being collected. (e.g. include/exclude regular expression for items such as network interfaces, disks, processes, file systems, etc.)

### Windows WMI collectors

* `cache`
    * config file `cache.(json|toml|yaml)`
    * options: only the common options
* `disk`
    * config file `disk.(json|toml|yaml)`
    * options:
        * `logical_disks` string(true|false), include logical disks (default "true")
        * `physical_disks` string(true|false), include physical disks (default "true")
        * `include_regex` string, regular expression for disk inclusion - default `.+`
        * `exclude_regex` string, regular expression for disk exclusion - default empty
* `memory`
    * config file `memory.(json|toml|yaml)`
    * options: only the common options
* `interface`
    * config file `interface.(json|toml|yaml)`
    * options:
        * `include_regex` string, regular expression for interface inclusion - default `.+`
        * `exclude_regex` string, regular expression for interface exclusion - default empty
* `ip`
    * config file `ip.(json|toml|yaml)`
    * options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* `tcp`
    * config file `tcp.(json|toml|yaml)`
    * options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* `udp`
    * config file `udp.(json|toml|yaml)`
    * options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* `objects`
    * config file `objects.(json|toml|yaml)`
    * options: only the common options
* `paging_file`
    * config file `paging_file.(json|toml|yaml)`
    * options:
        * `include_regex` string, regular expression for file inclusion - default `.+`
        * `exclude_regex` string, regular expression for file exclusion - default empty
* `processor`
    * config file `processor.(json|toml|yaml)`
    * options:
        * `report_all_cpus` string, include all cpus, not just total (default "true")
* `processes`
    * disabled by default (28 metrics _per_ process)
    * config file `processes.(json|toml|yaml)`
    * options:
        * `include_regex` string, regular expression for process inclusion - default `.+`
        * `exclude_regex` string, regular expression for process exclusion - default empty

### Linux ProcFS collectors

WIP

* `cpu`
