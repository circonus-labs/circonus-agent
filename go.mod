module github.com/circonus-labs/circonus-agent

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20211005130812-5bb3c17173e5
	github.com/alecthomas/units v0.0.0-20210927113745-59d0afb8317a
	github.com/bi-zone/wmi v1.1.4
	github.com/circonus-labs/circonus-gometrics/v3 v3.4.6
	github.com/circonus-labs/go-apiclient v0.7.15
	github.com/fatih/color v1.12.0 // indirect
	github.com/gojuno/minimock/v3 v3.0.10
	github.com/google/uuid v1.3.0
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0
	github.com/maier/go-appstats v0.2.0
	github.com/pelletier/go-toml v1.9.4
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.31.1
	github.com/rs/zerolog v1.25.0
	github.com/shirou/gopsutil/v3 v3.21.8
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.9.0
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211004093028-2c5d950f24ef
	gopkg.in/yaml.v2 v2.4.0
)

go 1.16
