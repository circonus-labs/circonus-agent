The state directory is used when `--check-enable-new-metrics` is activated.

It is important that the directory be *owned* by the user `circonus-agentd` will
run as (i.e. `nobody` on linux). This is required so that the daemon can maintain
state for the metrics and track which metrics are actually new versus having been
seen before and intentionally disabled.
