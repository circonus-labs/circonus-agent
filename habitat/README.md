# Habitat package: circonus-agent

## Description
A Habitat package to run the Circonus monitoring agent.

## Testing Builds
Install Habitat if needed, see https://www.habitat.sh/docs/install-habitat/
Enter the Studio to test locally:
```
hab studio enter
rebuild
echo 'api_key = "<circonus api key>"' | hab config apply circonus-agent.default $(date +%s)
```
Or, you can create a `studio.toml` file next to `default.toml` and add
```
api_key = "<circonus api key>""
```
to it. Check service status inside the Studio with the `sup-log` command.

## Deploying
A basic deploy can be accomplished by starting the Habitat Supervisor using the
init system of your choice. See https://www.habitat.sh/docs/best-practices/#running-habitat-servers

Once the Supervisor is running you can load this Habitat service like:
```
hab svc load <origin>/circonus-agent --strategy at-once
```
(Setting an update strategy with `--strategy` ensures that the service will
update itself whenever a new stable release appears.)

## Adding Custom Plugins
Custom plugins can be enabled by either forking this repository and creating
plugin scripts in the `plugins` directory or depending on this package using the
"wrapper package" pattern, where this package is included in your wrapper
package's `pkg_deps` array.

At build time, shebang interpreter lines in your plugins will be replaced with
fully-qualified paths to Habitat packages in your `pkg_deps` so that runtime
behavior can be guaranteed. For example,
```
#!/bin/bash
```
will be replaced with
```
#!/hab/pkgs/core/bash/4.3.42/20170513213519
```
The new value will be the path resolved from the `core/bash` you call as a
dependency in your Plan's `pkg_deps`.
