#!/usr/bin/env bash

# with shell conf config

conf="${PWD}/test4.conf"
[[ -f $conf ]] || { >&2 echo "Unable to find config '${conf}'"; exit 1; }

source $conf

printf "tm4%s_uint32\tI\t%d\n" $confid $(( (RANDOM % 100) +1 ))
