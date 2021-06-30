if [ -f /lib/systemd/system/circonus-agent.service ]; then
    /bin/systemctl enable circonus-agent
    /bin/systemctl start circonus-agent >/dev/null 2>&1
elif [ -f /etc/init.d/circonus-agent ]; then
    /sbin/chkconfig --add circonus-agent
    /sbin/service circonus-agent start >/dev/null 2>&1
fi
