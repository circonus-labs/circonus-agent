#!/bin/bash

function percent_used() {
	local total=$(free | grep Mem | awk '{print $2}')
	local used=$(free | grep Mem | awk '{print $3}')
	echo -n $(awk "BEGIN { pc=100*${used}/${total}; i=int(pc); print (pc-i<0.5)?i:i+1 }")
}

echo -e "percent_used\tn\t$(percent_used)"
