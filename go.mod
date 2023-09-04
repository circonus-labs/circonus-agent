module github.com/circonus-labs/circonus-agent

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20211005130812-5bb3c17173e5
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137
	github.com/bi-zone/wmi v1.1.4
	github.com/circonus-labs/circonus-gometrics/v3 v3.4.6
	github.com/circonus-labs/go-apiclient v0.7.23
	github.com/gojuno/minimock/v3 v3.1.3
	github.com/google/uuid v1.3.1
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.4
	github.com/maier/go-appstats v0.2.0
	github.com/openhistogram/circonusllhist v0.3.0
	github.com/pelletier/go-toml v1.9.5
	github.com/prometheus/client_model v0.4.0
	github.com/prometheus/common v0.44.0
	github.com/rs/zerolog v1.30.0
	github.com/shirou/gopsutil/v3 v3.23.7
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.16.0
	golang.org/x/sync v0.3.0
	golang.org/x/sys v0.12.0
	gopkg.in/yaml.v2 v2.4.0
)

go 1.16
