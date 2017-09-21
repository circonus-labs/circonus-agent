#!/usr/bin/env bash

# exit on failures
set -o errexit

runner="statsd-runner.sh"
plug_dir="${PWD}"
[[ -x "${plug_dir}/${runner}" ]] || plug_dir="${PWD}/statsd_test"
[[ -x "${plug_dir}/${runner}" ]] || { >&2 echo  "Unable to find runner '${runner}'"; exit 1; }

cmd="${plug_dir}/${runner}"

$cmd "host."
#$cmd "group."
#$cmd ""

# run the multiple metric submitter
cmd="${plug_dir}/statsd-multi.sh"
[[ -x $cmd ]] && $cmd
