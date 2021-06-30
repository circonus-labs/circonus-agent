#!/bin/bash

if [ -f /lib/systemd/system/circonus-agent.service ]; then
    /bin/systemctl disable circonus-agent
    /bin/systemctl stop circonus-agent >/dev/null 2>&1
elif [ -f /etc/init.d/circonus-agent ]; then
    /usr/sbin/update-rc.d circonus-agent remove
fi

exit 0
