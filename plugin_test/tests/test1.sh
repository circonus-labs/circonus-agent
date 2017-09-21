#!/usr/bin/env bash

# non symlink file

printf "tm1_uint64\tL\t%d\n" $(( (RANDOM % 100) +1 ))
