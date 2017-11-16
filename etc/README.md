# Circonus Agent

The `etc` directory is used for the main configuration of the Circonus agent, as well as, configuration files for builtin collectors.

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

---

# Collector configurations

Three formats are supported (json, toml, yaml) for collector configurations. Collector configurations should be stored in the `etc` directory where the agent was installed (for example: `/opt/circonus/agent/etc` or `C:\circonus-agent\etc`).

# Linux

## ProcFS collectors

All ProcFS collectors have a basic set of configuration options:

| Option                   | Type             | Default            | Description |
| ------------------------ | ---------------- | ------------------ | ----------- |
| `id`                     | string           | name of collector  | ID/Name of the collector (used as prefix for metrics). |
| `metrics_enabled`        | array of strings | empty              | list of metrics which are enabled (to be collected) |
| `metrics_disabled`       | array of strings | empty              | list of metrics which are disabled (should NOT be collected) |
| `metrics_default_status` | string           | `enabled`          | how a metric NOT in the enabled/disabled lists should be handled ("enabled" or "disabled") |
| `run_ttl`                | string           | empty              | indicating collector will run no more frequently than TTL (e.g. "10s", "5m", etc. - for expensive collectors) |

Additionally, each collector may have more configuration options specific to _what_ is being collected. (e.g. include/exclude regular expression for items such as network interfaces, disks, etc.)

* CPU
    * ID: `cpu`
    * Config file: `cpu_collector.(json|toml|yaml)`
    * Options:
        * `report_all_cpus` string, include all cpus, not just total (default "false")
* Disk stats
    * ID: `diskstats`
    * Config file: `diskstats_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for disk inclusion - default `.+`
        * `exclude_regex` string, regular expression for disk exclusion - default empty
* Network interfaces
    * ID: `if`
    * Config file: `if_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for interface inclusion - default `.+`
        * `exclude_regex` string, regular expression for interface exclusion - default `lo`
* Memory
    * ID: `vm`
    * Config file: `vm_collector.(json|toml|yaml)`
    * Options: only the common options
* System load
    * ID: `loadavg`
    * Config file: `loadavg_collector.(json|toml|yaml)`
    * Options: only the common options

# Windows

## WMI

All WMI collectors have a basic set of configuration options:

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

* Cache
    * ID: `cache`
    * Config file: `cache_collector.(json|toml|yaml)`
    * Options: only the common options
* Disk
    * ID: `disk`
    * Config file: `disk_collector.(json|toml|yaml)`
    * Options:
        * `logical_disks` string(true|false), include logical disks (default "true")
        * `physical_disks` string(true|false), include physical disks (default "true")
        * `include_regex` string, regular expression for disk inclusion - default `.+`
        * `exclude_regex` string, regular expression for disk exclusion - default empty
* Memory
    * ID: `memory`
    * Config file: `memory_collector.(json|toml|yaml)`
    * Options: only the common options
* Network interfaces
    * ID: `interface`
    * Config file: `interface_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for interface inclusion - default `.+`
        * `exclude_regex` string, regular expression for interface exclusion - default empty
* IP network protocol
    * ID: `ip`
    * Config file: `ip_collector.(json|toml|yaml)`
    * Options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* TCP network protocol
    * ID: `tcp`
    * Config file: `tcp_collector.(json|toml|yaml)`
    * Options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* UDP network protocol
    * ID: `udp`
    * Config file: `udp_collector.(json|toml|yaml)`
    * Options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* Objects
    * ID: `objects`
    * Config file: `objects_collector.(json|toml|yaml)`
    * Options: only the common options
* Paging file
    * ID: `paging_file`
    * Config file: `paging_file_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for file inclusion - default `.+`
        * `exclude_regex` string, regular expression for file exclusion - default empty
* Processors
    * ID: `processor`
    * Config file: `processor_collector.(json|toml|yaml)`
    * Options:
        * `report_all_cpus` string, include all cpus, not just total (default "true")
* Processes
    * ID: `processes`
    * NOTE: disabled by default (28 metrics _per_ process)
    * Config file: `processes_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for process inclusion - default `.+`
        * `exclude_regex` string, regular expression for process exclusion - default empty

# Common

## Prometheus collector

Collect from endpoints exposing prometheus formatted metrics. The Prometheus collector is enabled by default. It is automatically disabled if no configuration file is found.

ID: `prometheus`
Config file: `prometheus_collector.(json|toml|yaml)`
Options:

| Option                   | Type             | Default            | Description |
| ------------------------ | ---------------- | ------------------ | ----------- |
| `metrics_enabled`        | array of strings | empty              | list of metrics which are enabled (to be collected) |
| `metrics_disabled`       | array of strings | empty              | list of metrics which are disabled (should NOT be collected) |
| `metrics_default_status` | string           | `enabled`          | how a metric NOT in the enabled/disabled lists should be handled ("enabled" or "disabled") |
| `run_ttl`                | string           | empty              | indicating collector will run no more frequently than TTL (e.g. "10s", "5m", etc. - for expensive collectors) |
| `urls`                   | array of urldefs | empty              | required, without any URLs the collector is disabled |
| URL definition (urldefs) |||
| `id`                     | string           | empty              | required, used as prefix for metrics from this URL |
| `url`                    | string           | url                | required, URL which responds with Prometheus text format metrics |
| `ttl`                    | string           | `30s`              | optional, timeout for the request |
