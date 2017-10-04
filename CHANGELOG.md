# v0.2.0
    * add ability to create a [reverse] check - if a check bundle id is not provided for reverse, the agent will search for a suitable check bundle for the host. Previously, if a check bundle could not be found the agent would exit. Now, when `--reverse-create-check` is supplied, the agent has the ability to create a check, rather than exit.
    * expose basic app stats endpoint /stats
# v0.1.2
    * fix statsd packet channel (broken in v0.1.1)
    * update readme with current instructions
# v0.1.1
    * merge structs
    * eliminate race condition
# v0.1.0
    * add freebsd and solaris builds for testing
    * add more test coverage throughout
    * switch to tomb instead of contexts
    * refactor code throughout
    * add build constraints to control target specific signal handling in agent package
    * fix race condition w/inventory handler
    * reset connection attempts after successful send/receive (catch connection drops)
    * randomize connection retry attempt delays (not all agents retrying on same schedule)
# v0.0.3
    * integrate context
    * cleaner shutdown handling
# v0.0.2
    * move `circonus-agentd` binary to `sbin/circonus-agentd`
    * refactor (plugins, server, reverse, statsd)
    * add agent package
# v0.0.1
    * Initial development working release
