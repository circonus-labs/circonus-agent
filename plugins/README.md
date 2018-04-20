# Circonus Agent

Place plugins, and plugin specific configuration files, into this directory.

Alternatively, use the `--plugin-dir` command line argument to use an existing directory (e.g. NAD's `etc/node-agent.d` plugin directory).

## Plugins

* Are located in the `--plugin-dir`.
* Must be regular files or symlinks.
* Must be executable (e.g. `0755`)
* Files are expected to be named matching a pattern of: `<base_name>.<ext>` (e.g. `foo.sh`)
* Directories are ignored.
* Configuration files are ignored.
    * Configuration files are defined as files with extensions of `.json` or `.conf`
    * A `.json` file is assumed to be a configuration for a plugin with the same `base_name` (e.g. `foo.json` is a configuration for `foo.sh`, `foo.exe`, etc.)
        * JSON config files are loaded and arguments defined are passed to the plugin instance(s).
        * The format for JSON config files is: `{"instance_id": ["arg1", "arg2", ...], ...}`.
        * One instance of the plugin will be run for each distinct `instance_id` found in the JSON.
        * The format of the resulting metric names would be: **plugin\`instance_id\`metric_name**
    * A `.conf` file is assumed to be a shell configuration file which is loaded by the plugin itself (e.g. `foo.sh` contains a line `source foo.conf`).
* All other directory entries are ignored.

## Running plugin environment

When plugins are executed, the _current working directory_ will be set to the `--plugin-dir`, for relative path references to find configs or data files. Scripts may safely reference `$PWD`. See `plugin_test/write_test/wtest1.sh` for example. In `plugin_test`, run `ln -s write_test/wtest1.sh`, start the agent (e.g. `go run main.go -p plugin_test`), then `curl localhost:2609/` to see it in action.

## Plugin Output

Output from plugins is expected on `stdout` either tab-delimited or json.

## Metric types

Plugin output (whether json or tab-delimited) supports the following types:

| Type | Description             |
| ---- | ----------------------- |
| `i`  | signed 32-bit integer   |
| `I`  | unsigned 32-bit integer |
| `l`  | signed 64-bit integer   |
| `L`  | unsigned 64-bit integer |
| `n`  | double/float            |
| `s`  | string/text             |

### Tab delimited

`metric_name<TAB>metric_type<TAB>metric_value[<TAB>tag_list]`

The *tag_list* is optional, a comma separated list of key:value pairs to use as Stream Tags.

### JSON

```json
{
    "metric_name1": {
        "_type": "metric_type",
        "_value": "metric_value"
    },
    "metric_name2": {
        "_tags": ["cat1:val1,cat2:val2"],
        "_type": "metric_type",
        "_value": "metric_value"
    },
    ...
}
```

The JSON `_tags` attribute will be converted into stream tags format embedded into the metric name.
