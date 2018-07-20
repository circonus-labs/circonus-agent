# Linux IO specific plugins

## IO Latency

Produces the following metrics:

1. Time request spent in queue - `io_latency.queue_time`
2. Time for device to handle request - `io_latency.device_time`
3. Total request handling time - `io_latency.total_time`

NOTE: `io_latency.elf` must run as root.

Link `io_latency.elf` into the `plugins` directory. `ln -s linux/io/io_latency/io_latency.elf` and run the agent as root.

or

Create a small shell script in `plugins` which uses `sudo` to run `io_latency.elf`. Ensure that the user running the agent
has sudo access for the `io_latency.elf` binary.

Example `plugins/io_latency.sh` using sudo.
```
#!/usr/bin/env bash

sudo linux/io/io_latency/io_latency.elf
```
