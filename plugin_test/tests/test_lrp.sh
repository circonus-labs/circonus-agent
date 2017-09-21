#!/usr/bin/env bash

while true; do
    for i in {0..3}; do
        printf "test_lrp_%d_int32\ti\t%d\n" $i $(( (RANDOM % 100) +1 ))
    done
    echo
    sleep 60
done
