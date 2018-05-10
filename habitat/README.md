# Habitat package: circonus-agent

## Description
A Habitat package to run the Circonus monitoring agent.

## Usage
Install Habitat if needed:
```
curl https://raw.githubusercontent.com/habitat-sh/habitat/master/components/hab/install.sh | sudo bash
```
Set appropriate env vars and then enter the Studio to test locally:
```
export HAB_STUDIO_SECRET_CIRCONUS_AUTH_TOKEN=<your auth token>
hab studio enter
./build_and_run.sh
```
