module github.com/circonus-labs/circonus-agent

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20210608160410-67692ebc98de
	github.com/alecthomas/units v0.0.0-20210208195552-ff826a37aa15
	github.com/bi-zone/wmi v1.1.4
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/circonus-labs/circonus-gometrics/v3 v3.4.5
	github.com/circonus-labs/go-apiclient v0.7.15
	github.com/gojuno/minimock/v3 v3.0.9
	github.com/google/uuid v1.2.0
	github.com/hashicorp/go-hclog v0.10.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0
	github.com/maier/go-appstats v0.2.0
	github.com/pelletier/go-toml v1.9.3
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.29.0
	github.com/rs/zerolog v1.23.0
	github.com/shirou/gopsutil/v3 v3.21.7
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210630005230-0f9fa26af87c
	gopkg.in/yaml.v2 v2.4.0
)

go 1.16
