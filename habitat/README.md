# Habitat package: circonus-agent

## Description
A Habitat package to run the Circonus monitoring agent.

## Testing Builds
Install Habitat if needed:
```
curl https://raw.githubusercontent.com/habitat-sh/habitat/master/components/hab/install.sh | sudo bash
```

Enter the Studio to test locally:
```
hab studio enter
rebuild
echo 'api_key = "<circonus api key>"' \
  | hab config apply circonus-agent.default $(date +%s)
```
Check service status inside the Studio with the `sup-log` command.

## Deploying
A basic deploy can be accomplished by starting the Habitat Supervisor using the
init system of your choice. A systemd service definitionn would look something
like:
```
[Unit]
Description=The Habitat Supervisor

[Service]
ExecStart=/bin/hab sup run

[Install]
WantedBy=default.target
```
Once the Supervisor is running you can add this Habitat service to it like this:
```
hab start bixu/circonus-agent --strategy at-once
```
(Setting an update strategy with `--strategy` ensures that the service will
update itself whenever a new stable release appears.)
