#!/usr/bin/env bash

# exit on failures
set -o errexit

statsd="statsd.sh"
plug_dir="${PWD}"
[[ -x "${plug_dir}/${statsd}" ]] || plug_dir="${PWD}/statsd_test"
[[ -x "${plug_dir}/${statsd}" ]] || { >&2 echo  "Unable to find statsd '${statsd}'"; exit 1; }


cmd="${plug_dir}/${statsd}"

# pfx is the metric destination type, matching host prefix or group prefix as configured
# host.
# group.
# or nothing, e.g. "" (will go to host, default)
pfx="${1:-}"

metric="${pfx}test_c"
#echo "counter ${metric}"
$cmd "${metric}:1|c"
#sleep 1

v=$(( ( RANDOM % 10 )  + 1 ))
metric="${pfx}test_g"
#echo "gauge $v ${metric}"
$cmd "${metric}:${v}|g"
#sleep 1

v=$(( ( RANDOM % 10 )  + 1 ))
metric="${pfx}test_ms"
#echo "timer $v ${metric}"
$cmd "${metric}:${v}|ms"
#sleep 1

v=$(( ( RANDOM % 10 )  + 1 ))
metric="${pfx}test_h"
#echo "histogram $v ${metric}"
$cmd "${metric}:${v}|h"
#sleep 1

if (( $v % 2 )); then
	t="foo"
else
	t="bar"
fi

metric="${pfx}test_s"
#echo "set $v ${metric}"
$cmd "${metric}:${t}|s"
#sleep 1

metric="${pfx}test_t"
#echo "text $t ${metric}"
$cmd "${metric}:${t}|t"
