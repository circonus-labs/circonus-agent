#!/usr/bin/env bash

# bad metric type
printf "tm5_bad_type\tQ\t%d\n" $(( (RANDOM % 100) +1 ))

# no delimeters
printf "tm5_bad_delim1 i %d\n" $(( (RANDOM % 100) +1 ))

# only 1 delimeter
printf "tm5_bad_delim2\t\n"

# bad number of fields (few)
printf "tm5_bad_fields1\n"

# bad number of fields (many)
printf "tm5_bad_fields2\ti\t0\tfoo:bar,herp:derp\n"

# invalid number
printf "tm5_not_number\ti\tfoo\n"
