module github.com/circonus-labs/circonus-agent

// NOTE: github.com/shirou/gopsutil does semver incorrectly (leading zeros
//       on patch level to represent month) the semver spec specifically
//       states NO LEADING ZEROS (https://semver.org/#spec-item-2).
//       To work around this, go get github.com/shirou/gopsutil@<commit id>
//       for release. e.g. for release v2.19.04 the releases page indicates
//       it was commit id fa98459, so use the following command:
//            `go get github.com/shirou/gopsutil@fa98459`

require (
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d
	github.com/alecthomas/units v0.0.0-20151022065526-2efee857e7cf
	github.com/circonus-labs/circonus-gometrics/v3 v3.0.0-beta.4
	github.com/circonus-labs/circonusllhist v0.1.3
	github.com/circonus-labs/go-apiclient v0.6.4
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/gojuno/minimock/v3 v3.0.0
	github.com/hashicorp/go-retryablehttp v0.5.4 // indirect
	github.com/magiconair/properties v1.8.1 // indirect
	github.com/maier/go-appstats v0.2.0
	github.com/pelletier/go-toml v1.4.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.6.0
	github.com/rs/zerolog v1.14.3
	github.com/shirou/gopsutil v0.0.0-20190427031343-fa9845945e5b
	github.com/spf13/afero v1.2.2 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/viper v1.4.0
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb
	golang.org/x/text v0.3.2 // indirect
	gopkg.in/yaml.v2 v2.2.2
)
