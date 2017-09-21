#!/usr/bin/env bash

# multiple metric sets, tests OVERWRITING for each blank line
# metrics are not aggregated, last set wins...

emit() {
    for i in {0..10}; do
        printf "tm8_%d_%d_dbl\tn\t%f\n" $1 $i $(echo $(( (RANDOM % 100) +1 ))".$(( (RANDOM % 99) +1 ))")
    done
}

emit 1
echo # trigger processing

sleep 1

emit 2
echo # trigger processing

emit 3

# script exits, to trigger final processing - within a given interval, the last output wins
