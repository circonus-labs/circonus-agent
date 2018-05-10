#!/bin/bash

echo -e  "1min\tn\t$(cat /proc/loadavg | awk '{print $1}')"
echo -e  "5min\tn\t$(cat /proc/loadavg | awk '{print $2}')"
echo -e "10min\tn\t$(cat /proc/loadavg | awk '{print $3}')"
