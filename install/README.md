# Circonus Agent Installer

A small, basic installer script for the circonus agent.

## Notes

* Currently works with `rpm` and `deb` packages for amd64, see [latest release](https://github.com/circonus-labs/circonus-agent/releases/latest)
* A valid Circonus API token key is required
* Requires access to the internet ([github](github.com) and Circonus)
* Works with Circonus SaaS (not inside deployments)
* More options and flexibility will be added as needed

## Options

```sh
Circonus Agent Install Help

Usage

  install.sh --key <apikey>

Options

  --key           Circonus API key/token **REQUIRED**
  [--app]         Circonus API app name (authorized w/key) Default: circonus-agent
  [--broker]      Circonus Broker ID (_cid from broker api object|select) Default: select
  [--ver]         Circonus Agent version tag (e.g. v2.3.2) Default: latest release
  [--os]          Install for specific os if agent unable to detect 
                  (el8|el7|el6|ubuntu.20.04|ubuntu.18.04|ubuntu.16.04)
  [--help]        This message

Note: Provide an authorized app for the key or ensure api
      key/token has adequate privileges (default app state:allow)
```

## Examples

### With only a key (ensure, key has default app state set to allow)

```sh
curl -sSL "https://raw.githubusercontent.com/circonus-labs/circonus-agent/master/install/install.sh" | bash -s -- --key <circonus api key>
```

### With key and explicit app name already allowed with key

```sh
curl -sSL "https://raw.githubusercontent.com/circonus-labs/circonus-agent/master/install/install.sh" | bash -s -- --key <circonus api key> --app <app named for key>
```

### With key and explicit version

```sh
curl -sSL "https://raw.githubusercontent.com/circonus-labs/circonus-agent/master/install/install.sh" | bash -s -- --key <circonus api key> --ver v2.3.2
```

### With key and explicit broker

Note: In Circonus UI, navigate to _Integrations>Brokers>View>API object_ to see the broker `_cid`.

```sh
curl -sSL "https://raw.githubusercontent.com/circonus-labs/circonus-agent/master/install/install.sh" | bash -s -- --key <circonus api key> --broker 2
```
