#!/usr/bin/env bash

# exit on failures
set -o errexit

data_dir="${PWD}"
[[ -f "${data_dir}/types.json" ]] || data_dir="${PWD}/write_test"
[[ -f "${data_dir}/types.json" ]] || { >&2 echo "Unable to find 'types.json'"; exit 1; }

# simple PUT
curl -sSL -X PUT -H 'Content-Type: application/json' -d '{"write_put_simple":{"_type": "i", "_value": 1}}' 'http://127.0.0.1:2609/write/wt_put'

# simple POST
curl -sSL -X POST -H 'Content-Type: application/json' -d '{"write_put_simple":{"_type": "i", "_value": 1}}' 'http://127.0.0.1:2609/write/wt_post'

# example with each metric type
curl -sSL -X PUT -H 'Content-Type: application/json' -d @${data_dir}/types.json 'http://127.0.0.1:2609/write/wt_metric_types'
