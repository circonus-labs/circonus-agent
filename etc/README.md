# Circonus Agent

The `etc` directory is used for the main configuration of the Circonus agent, as well as, configuration files for builtin collectors.

# Main configuration

File name: `circonus-agent.(json|toml|yaml)`

An example configuration, with default values, can be retrieved using the `--show-config=(json|toml|yaml)`.

## Configuration file quick start

Run one of the following (from the base directory where the agent was installed) and edit the resulting configuration file:


```
sbin/circonus-agentd --show-config=json > etc/circonus-agent.json.tmp
sbin/circonus-agentd --show-config=toml > etc/circonus-agent.toml.tmp
sbin/circonus-agentd --show-config=yaml > etc/circonus-agent.yaml.tmp
```

or, on Windows:

```
sbin\circonus-agentd.exe --show-config=json > etc\circonus-agent.json.tmp
sbin\circonus-agentd.exe --show-config=toml > etc\circonus-agent.toml.tmp
sbin\circonus-agentd.exe --show-config=yaml > etc\circonus-agent.yaml.tmp
```

Edit the resulting file to customize configuration settings. When done, rename file to remove the `.tmp` extension. (e.g. `mv etc/circonus-agent.json.tmp` `etc/circonus-agent.json`)

---

# Builtin Collector Configurations

Three formats are supported (json, toml, yaml) for collector configurations. Collector configurations should be stored in the `etc` directory where the agent was installed (for example: `/opt/circonus/agent/etc` or `C:\circonus-agent\etc`).

## Default collectors:

* Linux: `['procfs/cpu', 'procfs/disk', 'procfs/if', 'procfs/load', 'procfs/proto', 'procfs/vm']`
* Windows: `['wmi/cache', 'wmi/disk', 'wmi/ip', 'wmi/interface', 'wmi/memory', 'wmi/object', 'wmi/paging_file' 'wmi/processor', 'wmi/tcp', 'wmi/udp']`
* Generic: `['generic/cpu', 'generic/disk', 'generic/fs', 'generic/if', 'generic/load', 'generic/proto', 'generic/vm']`
* Common `prometheus` (disabled if no configuration file exists)

# Linux

## ProcFS collectors

All ProcFS collectors have a basic set of configuration options:

| Option                   | Type             | Default            | Description |
| ------------------------ | ---------------- | ------------------ | ----------- |
| `id`                     | string           | name of collector  | ID/Name of the collector (used as prefix for metrics). |
| `run_ttl`                | string           | empty              | indicating collector will run no more frequently than TTL (e.g. "10s", "5m", etc. - for expensive collectors) |

Additionally, each collector may have more configuration options specific to _what_ is being collected. (e.g. include/exclude regular expression for items such as network interfaces, disks, etc.)

Example usage: `--collectors="procfs/cpu,procfs/disk,procfs/if,procfs/load,procfs/vm"`

* CPU
    * ID: `procfs/cpu`
    * Config file: `procfs_cpu_collector.(json|toml|yaml)`
    * Options:
        * `report_all_cpus` string, include all cpus, not just total (default "false")
* Disk stats
    * ID: `procfs/disk`
    * Config file: `procfs_disk_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for disk inclusion - default `.+`
        * `exclude_regex` string, regular expression for disk exclusion - default empty
* Network interfaces
    * ID: `procfs/if`
    * Config file: `procfs_if_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for interface inclusion - default `.+`
        * `exclude_regex` string, regular expression for interface exclusion - default `lo`
* Memory
    * ID: `procfs/vm`
    * Config file: `procfs_vm_collector.(json|toml|yaml)`
    * Options: _only the common options_
    * Note: agent v1 difference from NAD `vm.sh` - htop calculations are used for memory to better represent free/used
* System load
    * ID: `procfs/load`
    * Config file: `procfs_load_collector.(json|toml|yaml)`
    * Options: _only the common options_

# Windows

## WMI

All WMI collectors have a basic set of configuration options:

| Option                   | Type             | Default            | Description |
| ------------------------ | ---------------- | ------------------ | ----------- |
| `id`                     | string           | name of collector  | ID/Name of the collector (used as prefix for metrics). |
| `metric_name_regex`      | string           | `[^a-zA-Z0-9.-_:]` | regular expression of valid characters for the metric names |
| `metric_name_char`       | string           | `_`                | used for replacing invalid characters in a metric name (those not matching `metric_name_regex`) |
| `run_ttl`                | string           | empty              | indicating collector will run no more frequently than TTL (e.g. "10s", "5m", etc. - for expensive collectors) |

Additionally, each collector may have more configuration options specific to _what_ is being collected. (e.g. include/exclude regular expression for items such as network interfaces, disks, processes, file systems, etc.)

Example usage: `--collectors="wmi/cache,wmi/disk,wmi/memory,wmi/interface,wmi/ip,wmi/tcp,wmi/udp,wmi/objects,wmi/processor,wmi/processes"`

* Cache
    * ID: `wmi/cache`
    * Config file: `wmi_cache_collector.(json|toml|yaml)`
    * Options: only the common options
* Disk
    * ID: `wmi/disk`
    * Config file: `wmi_disk_collector.(json|toml|yaml)`
    * Options:
        * `logical_disks` string(true|false), include logical disks (default "true")
        * `physical_disks` string(true|false), include physical disks (default "true")
        * `include_regex` string, regular expression for disk inclusion - default `.+`
        * `exclude_regex` string, regular expression for disk exclusion - default empty
* Memory
    * ID: `wmi/memory`
    * Config file: `wmi_memory_collector.(json|toml|yaml)`
    * Options: only the common options
* Network interfaces
    * ID: `wmi/interface`
    * Config file: `wmi_interface_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for interface inclusion - default `.+`
        * `exclude_regex` string, regular expression for interface exclusion - default empty
* IP network protocol
    * ID: `wmi/ip`
    * Config file: `wmi_ip_collector.(json|toml|yaml)`
    * Options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* TCP network protocol
    * ID: `wmi/tcp`
    * Config file: `wmi_tcp_collector.(json|toml|yaml)`
    * Options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* UDP network protocol
    * ID: `wmi/udp`
    * Config file: `wmi_udp_collector.(json|toml|yaml)`
    * Options:
        * `enable_ipv4` string(true|false), include IPv4 metrics - default "true"
        * `enable_ipv6` string(true|false), include IPv6 metrics - default "true"
* Objects
    * ID: `wmi/objects`
    * Config file: `wmi_objects_collector.(json|toml|yaml)`
    * Options: _only the common options_
* Paging file
    * ID: `wmi/paging_file`
    * Config file: `wmi_paging_file_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for file inclusion - default `.+`
        * `exclude_regex` string, regular expression for file exclusion - default empty
* Processors
    * ID: `wmi/processor`
    * Config file: `wmi_processor_collector.(json|toml|yaml)`
    * Options:
        * `report_all_cpus` string, include all cpus, not just total (default "true")
* Processes
    * ID: `wmi/processes`
    * NOTE: disabled by default (28 metrics _per_ process)
    * Config file: `wmi_processes_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for process inclusion - default `.+`
        * `exclude_regex` string, regular expression for process exclusion - default empty

# Generic collectors

All Generic collectors have a basic set of configuration options:

| Option                   | Type             | Default            | Description |
| ------------------------ | ---------------- | ------------------ | ----------- |
| `id`                     | string           | name of collector  | ID/Name of the collector (used as prefix for metrics). |
| `run_ttl`                | string           | empty              | indicating collector will run no more frequently than TTL (e.g. "10s", "5m", etc. - for expensive collectors) |

Additionally, each collector may have more configuration options specific to _what_ is being collected. (e.g. include/exclude regular expression for items such as network interfaces, disks, filesystems, devices, etc.)

Example usage: `--collectors="generic/cpu,generic/disk,generic/fs,generic/load,generic/if,generic/proto,generic/vm"`

* CPU
    * ID: `generic/cpu`
    * Config file: `generic_cpu_collector.(json|toml|yaml)`
    * Options:
        * `report_all_cpus` string, include all cpus, not just total (default "false")
* Disk
    * ID: `generic/disk`
    * Config file: `generic_disk_collector.(json|toml|yaml)`
    * Options:
        * `io_devices` list of strings, specific devices to include (default <empty list> == all devices)
* Filesystem
    * ID: `generic/fs`
    * Config file: `generic_fs_collector.(json|toml|yaml)`
    * Options:
        * `include_fs_regex` string, regular expression for filesystem mount point inclusion - default `.+`
        * `exclude_fs_regex` string, regular expression for filesystem mount point exclusion - default empty
        * `exclude_fs_type` list of strings, specific filesystem types to exclude (default <empty list> == include all types)
        * `include_all_devices` string, include all devices, not just "drives" (default "false")
* Load
    * ID: `generic/load`
    * Config file: `generic_load_collector.(json|toml|yaml)`
    * Options: _only the common options_
* Network interfaces
    * ID: `generic/if`
    * Config file: `generic_if_collector.(json|toml|yaml)`
    * Options:
        * `include_regex` string, regular expression for interface inclusion - default `.+`
        * `exclude_regex` string, regular expression for interface exclusion - default `lo`
* Network protocols
    * ID: `generic/proto`
    * Config file: `generic_proto_collector.(json|toml|yaml)`
    * Options:
        * `protocols` list of strings, specific network protocols to exclude (default <empty list> == include all "ip,icmp,icmpmsg,tcp,udp,udplite")
* Virtual Memory
    * ID: `generic/vm`
    * Config file: `generic_vm_collector.(json|toml|yaml)`
    * Options: _only the common options_

# Common

## Prometheus collector

Collect from endpoints exposing prometheus formatted metrics. The Prometheus collector is enabled by default. It is automatically disabled if no configuration file is found.

ID: `prometheus`
Config file: `prometheus_collector.(json|toml|yaml)`
Options:

| Option                   | Type             | Default            | Description |
| ------------------------ | ---------------- | ------------------ | ----------- |
| `run_ttl`                | string           | empty              | indicating collector will run no more frequently than TTL (e.g. "10s", "5m", etc. - for expensive collectors) |
| `urls`                   | array of urldefs | empty              | required, without any URLs the collector is disabled |
| URL definition (urldefs) |||
| `id`                     | string           | empty              | required, used as prefix for metrics from this URL |
| `url`                    | string           | url                | required, URL which responds with Prometheus text format metrics |
| `ttl`                    | string           | `30s`              | optional, timeout for the request |
