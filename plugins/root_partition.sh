#!/bin/bash

function percent_used() {
	echo -n $(df --output=pcent "${1}" | grep -v Use | cut -d% -f1 | tr -d '[:space:]')
}

echo -e "percent_used\tn\t$(percent_used "/")"
