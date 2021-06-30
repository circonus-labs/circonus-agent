if [ $1 = 0 ]; then
    if [ -f /lib/systemd/system/circonus-agent.service ]; then
        /bin/systemctl disable circonus-agent
        /bin/systemctl stop circonus-agent >/dev/null 2>&1
    elif [ -f /etc/init.d/circonus-agent ]; then
        /sbin/chkconfig --del circonus-agent
        /sbin/service circonus-agent stop >/dev/null 2>&1
    fi
fi
exit 0
