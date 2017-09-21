#!/usr/bin/env bash

# with json config

if [[ "$1" == "" ]]; then
    >&2 echo "missing command line argument"
    exit 1
fi

printf "tm3%s_int32\ti\t%d\n" $1 $(( (RANDOM % 100) +1 ))
