#!/bin/bash

if [ -f /lib/systemd/system/circonus-agent.service ]; then
    /bin/systemctl enable circonus-agent
    /bin/systemctl start circonus-agent >/dev/null 2>&1
elif [ -f /etc/init.d/circonus-agent ]; then
    /usr/sbin/update-rc.d circonus-agent defaults 98 02
    /etc/init.d/circonus-agent start
fi
