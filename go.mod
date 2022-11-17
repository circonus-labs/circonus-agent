module github.com/circonus-labs/circonus-agent

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20211005130812-5bb3c17173e5
	github.com/alecthomas/units v0.0.0-20210927113745-59d0afb8317a
	github.com/bi-zone/wmi v1.1.4
	github.com/circonus-labs/circonus-gometrics/v3 v3.4.6
	github.com/circonus-labs/go-apiclient v0.7.20
	github.com/gojuno/minimock/v3 v3.0.10
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.1
	github.com/maier/go-appstats v0.2.0
	github.com/pelletier/go-toml v1.9.5
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.37.0
	github.com/rs/zerolog v1.28.0
	github.com/shirou/gopsutil/v3 v3.22.7
	github.com/spf13/cobra v1.5.0
	github.com/spf13/viper v1.12.0
	golang.org/x/sync v0.0.0-20220513210516-0976fa681c29
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a
	gopkg.in/yaml.v2 v2.4.0
)

go 1.16
