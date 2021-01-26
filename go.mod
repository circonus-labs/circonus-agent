module github.com/circonus-labs/circonus-agent

require (
	code.cloudfoundry.org/bytefmt v0.0.0-20200131002437-cf55d5288a48
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d
	github.com/alecthomas/units v0.0.0-20201120081800-1786d5ef83d4
	github.com/circonus-labs/circonus-gometrics/v3 v3.3.1
	github.com/circonus-labs/circonusllhist v0.1.4
	github.com/circonus-labs/go-apiclient v0.7.10
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/gojuno/minimock/v3 v3.0.8
	github.com/google/uuid v1.1.4
	github.com/hashicorp/go-hclog v0.10.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.6.8
	github.com/maier/go-appstats v0.2.0
	github.com/onsi/ginkgo v1.14.0 // indirect
	github.com/pelletier/go-toml v1.8.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.15.0
	github.com/rs/zerolog v1.20.0
	github.com/shirou/gopsutil v3.20.12+incompatible
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cast v1.3.1 // indirect
	github.com/spf13/cobra v1.1.1
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.7.1
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a
	golang.org/x/sys v0.0.0-20210105210732-16f7687f5001
	gopkg.in/ini.v1 v1.51.1 // indirect
	gopkg.in/yaml.v2 v2.4.0
)

go 1.15
