module github.com/circonus-labs/circonus-agent

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20200131002437-cf55d5288a48
	github.com/alecthomas/units v0.0.0-20210208195552-ff826a37aa15
	github.com/bi-zone/wmi v1.1.4
	github.com/circonus-labs/circonus-gometrics/v3 v3.4.5
	github.com/circonus-labs/go-apiclient v0.7.15
	github.com/gojuno/minimock/v3 v3.0.8
	github.com/google/uuid v1.2.0
	github.com/hashicorp/go-hclog v0.10.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.0
	github.com/maier/go-appstats v0.2.0
	github.com/onsi/ginkgo v1.14.0 // indirect
	github.com/pelletier/go-toml v1.9.3
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.23.0
	github.com/rs/zerolog v1.23.0
	github.com/shirou/gopsutil/v3 v3.21.5
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.8.1
	github.com/tklauser/go-sysconf v0.3.5 // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20210510120138-977fb7262007
	gopkg.in/yaml.v2 v2.4.0
)

go 1.16
