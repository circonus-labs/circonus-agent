#!/usr/bin/env bash

# multiple metrics

for i in {0..10}; do
    printf "tm7_%d_int64\tl\t%d\n" $i $(( (RANDOM % 100) +1 ))
done
