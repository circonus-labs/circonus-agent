# Circonus Agent service configurations

## Systemd

### Basic installation

```
# cp circonus-agent.service /usr/lib/systemd/system/circonus-agent.service
# systemctl enable circonus-agent
# systemctl start circonus-agent
# systemctl status circonus-agent
```

### Alternatives

#### Replace existing NAD installation performed via COSI

Edit the `ExecStart` line in the service configuration as follows:

```
ExecStart=/opt/circonus/agent/sbin/circonus-agentd --plugin-dir=/opt/circonus/nad/etc/node-agent.d --reverse --api-key=cosi
```

#### Barebones installation without plugins

This will start the agent and it will create its own check. Edit `ExecStart` line in the service configuration as follows:

```
ExecStart=/opt/circonus/agent/sbin/circonus-agentd --check-create --reverse --api-key=<ADD KEY> --api-app=<ADD APP>
```
