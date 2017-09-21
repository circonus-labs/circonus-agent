#!/usr/bin/env bash
host="${STATSD_HOST:-127.0.0.1}"
port="${STATSD_PORT:-8125}"

# Setup UDP socket with statsd server
exec 3<> /dev/udp/$host/$port

# Send data
printf "host.mtest1:1|c\nhost.mtest2:1|c\nhost.mtest3:2|g\nhost.mtest4:2|g" >&3

# Close UDP socket
exec 3<&-
exec 3>&-
