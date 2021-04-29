// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build windows

package nvidia

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/circonus-labs/circonus-agent/internal/builtins/collector"
	"github.com/circonus-labs/circonus-agent/internal/config"
	"github.com/circonus-labs/circonus-agent/internal/tags"
	cgm "github.com/circonus-labs/circonus-gometrics/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

// GPU metrics from the Windows Management Interface (wmi).
type GPU struct {
	exePath      string
	exeArgs      []string
	metadataList []gpuMeta
	metricList   []gpuMetric
	common
	interval time.Duration
}

// gpuOptions defines what elements can be overridden in a config file.
type gpuOptions struct {
	ID       string      `json:"id" toml:"id" yaml:"id"`
	ExePath  string      `mapstructure:"exe_path" json:"exe_path" toml:"exe_path" yaml:"exe_path"`
	Interval string      `mapstructure:"interval" json:"interval" toml:"interval" yaml:"interval"`
	Metrics  []gpuMetric `mapstructure:"metrics" json:"metrics" toml:"metrics" yaml:"metrics"`
	Metadata []gpuMeta   `mapstructure:"metadta" json:"metadata" toml:"metadata" yaml:"metadata"`
}

type gpuMetric struct {
	MatchValue interface{} `mapstructure:"match_value" json:"match_value" toml:"match_value" yaml:"match_value"` // so text metrics can be put into histograms. type th1 string if match, value 1, otherwise 0; type th2 string list, index is value, -1 if not found
	ArgName    string      `mapstructure:"arg_name" json:"arg_name" toml:"arg_name" yaml:"arg_name"`
	MetricName string      `mapstructure:"metric_name" json:"metric_name" toml:"metric_name" yaml:"metric_name"`
	MetricType string      `mapstructure:"metric_type" json:"metric_type" toml:"metric_type" yaml:"metric_type"`
	Units      string      `mapstructure:"units" json:"units" toml:"units" yaml:"units"`
}

type gpuMeta struct {
	ArgName string `mapstructure:"arg_name" json:"arg_name" toml:"arg_name" yaml:"arg_name"`
	TagName string `mapstructure:"tag_name" json:"tag_name" toml:"tag_name" yaml:"tag_name"`
}

// logshim is used to satisfy apiclient Logger interface (avoiding ptr receiver issue).
type logshim struct {
	logh zerolog.Logger
}

func (l logshim) Printf(fmt string, v ...interface{}) {
	l.logh.Printf(fmt, v...)
}

// NewGPUCollector creates new wmi collector.
func NewGPUCollector(cfgBaseName string) (collector.Collector, error) {
	c := GPU{}
	c.id = "gpu"
	c.common.pkgID = pkgName + "." + c.id
	c.logger = log.With().Str("pkg", pkgName).Str("id", c.id).Logger()
	c.common.baseTags = tags.FromList(tags.GetBaseTags())
	c.common.running = false

	c.exePath = filepath.Join("C:", string(os.PathSeparator), "Program Files", "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe")
	c.interval = 500 * time.Millisecond
	c.metadataList = []gpuMeta{
		{
			ArgName: "driver_version",
		},
		{
			ArgName: "name",
		},
		{
			ArgName: "uuid",
		},
	}
	c.metricList = []gpuMetric{
		{
			ArgName:    "fan.speed",
			MetricType: "h",
			Units:      "percent",
		},
		{
			ArgName:    "pstate",
			MetricType: "th2",
			MatchValue: []string{"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10", "P11", "P12"},
		},
		{
			ArgName:    "clocks_throttle_reasons.gpu_idle",
			MetricType: "th1",
			MatchValue: "Active",
		},
		{
			ArgName:    "clocks_throttle_reasons.applications_clocks_setting",
			MetricType: "th1",
			MatchValue: "Active",
		},
		{
			ArgName:    "clocks_throttle_reasons.sw_power_cap",
			MetricType: "th1",
			MatchValue: "Active",
		},
		{
			ArgName:    "clocks_throttle_reasons.hw_slowdown",
			MetricType: "th1",
			MatchValue: "Active",
		},
		{
			ArgName:    "clocks_throttle_reasons.hw_thermal_slowdown",
			MetricType: "th1",
			MatchValue: "Active",
		},
		{
			ArgName:    "clocks_throttle_reasons.hw_power_brake_slowdown",
			MetricType: "th1",
			MatchValue: "Active",
		},
		{
			ArgName:    "clocks_throttle_reasons.sw_thermal_slowdown",
			MetricType: "th1",
			MatchValue: "Active",
		},
		{
			ArgName:    "clocks_throttle_reasons.sync_boost",
			MetricType: "th1",
			MatchValue: "Active",
		},
		{
			ArgName:    "memory.used",
			MetricType: "h",
			Units:      "MiB",
		},
		{
			ArgName:    "memory.free",
			MetricType: "h",
			Units:      "MiB",
		},
		{
			ArgName:    "compute_mode",
			MetricType: "th2",
			MatchValue: []string{"Default", "Exclusive_Process", "Prohibited"},
		},
		{
			ArgName:    "utilization.gpu",
			MetricType: "h",
			Units:      "percent",
		},
		{
			ArgName:    "utilization.memory",
			MetricType: "h",
			Units:      "percent",
		},
		{
			ArgName:    "encoder.stats.sessionCount",
			MetricType: "h",
		},
		{
			ArgName:    "encoder.stats.averageFps",
			MetricType: "h",
		},
		{
			ArgName:    "encoder.stats.averageLatency",
			MetricType: "h",
		},
		{
			ArgName:    "temperature.gpu",
			MetricType: "h",
			Units:      "C",
		},
		{
			ArgName:    "temperature.memory",
			MetricType: "h",
			Units:      "C",
		},
		{
			ArgName:    "power.draw",
			MetricType: "h",
			Units:      "watts",
		},
		{
			ArgName:    "power.limit",
			MetricType: "h",
			Units:      "watts",
		},
		{
			ArgName:    "clocks.gr",
			MetricType: "h",
			Units:      "MHz",
		},
		{
			ArgName:    "clocks.sm",
			MetricType: "h",
			Units:      "MHz",
		},
		{
			ArgName:    "clocks.mem",
			MetricType: "h",
			Units:      "MHz",
		},
		{
			ArgName:    "clocks.video",
			MetricType: "h",
			Units:      "MHz",
		},
	}

	cmc := &cgm.Config{
		Debug: viper.GetBool(config.KeyDebugCGM),
		Log:   logshim{logh: c.logger.With().Str("pkg", "cgm.nvidia").Logger()},
	}
	// put cgm into manual mode (no interval, no api key, invalid submission url)
	cmc.Interval = "0"                            // disable automatic flush
	cmc.CheckManager.Check.SubmissionURL = "none" // disable check management (create/update)

	hm, err := cgm.NewCirconusMetrics(cmc)
	if err != nil {
		return nil, fmt.Errorf("nvidia cgm: %w", err)
	}

	c.common.metrics = hm

	if cfgBaseName == "" {
		return &c, nil
	}

	haveCfg := true
	var cfg gpuOptions
	if err := config.LoadConfigFile(cfgBaseName, &cfg); err != nil {
		if strings.Contains(err.Error(), "no config found matching") {
			haveCfg = false
			// return &c, nil
		} else {
			c.logger.Debug().Err(err).Str("file", cfgBaseName).Msg("loading config file")
			return nil, fmt.Errorf("%s config: %w", c.pkgID, err)
		}
	}

	if haveCfg {
		c.logger.Debug().Interface("config", cfg).Msg("loaded config")

		if cfg.ID != "" {
			c.id = cfg.ID
		}

		if cfg.ExePath != "" {
			c.exePath = cfg.ExePath
		}

		if cfg.Interval != "" {
			d, err := time.ParseDuration(cfg.Interval)
			if err != nil {
				return nil, fmt.Errorf("parsing interval: %w", err)
			}
			c.interval = d
		}

		if len(cfg.Metrics) != 0 {
			c.metricList = cfg.Metrics
		}
		if len(cfg.Metadata) != 0 {
			c.metadataList = cfg.Metadata
		}
	}

	gpuQueryArgs := make([]string, len(c.metricList))
	for i, a := range c.metricList {
		gpuQueryArgs[i] = a.ArgName
	}

	c.exeArgs = []string{
		"--format=csv,nounits,noheader",
		fmt.Sprintf("--loop-ms=%d", c.interval.Milliseconds()),
		fmt.Sprintf("--query-gpu=%s", strings.Join(gpuQueryArgs, ",")),
	}

	if err := c.tagMetadata(); err != nil {
		return nil, err
	}

	// collect metadata, this is a one and done run of the tool
	// add to c.baseTags

	return &c, nil
}

func (gpu *GPU) tagMetadata() error {
	if len(gpu.metadataList) == 0 {
		return nil
	}
	tagNames := make([]string, len(gpu.metadataList))
	metadata := make([]string, len(gpu.metadataList))
	for i, a := range gpu.metadataList {
		metadata[i] = a.ArgName
		tagNames[i] = a.TagName
		if tagNames[i] == "" {
			tagNames[i] = a.ArgName
		}
	}
	cmdArgs := []string{
		"--format=csv,nounits,noheader",
		fmt.Sprintf("--query-gpu=%s", strings.Join(metadata, ",")),
	}

	out, err := exec.Command(gpu.exePath, cmdArgs...).Output() //nolint:gosec
	if err != nil {
		return fmt.Errorf("getting gpu metadata: %w", err)
	}

	r := csv.NewReader(bytes.NewBuffer(out))
	records, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("parsing gpu metadata: %w", err)
	}

	if len(records) != 1 {
		return fmt.Errorf("invalid metadata %v", records) //nolint:goerr113
	}
	if len(records[0]) != len(tagNames) {
		return fmt.Errorf("metadata mismatch expected %d, got %d", len(tagNames), len(records)) //nolint:goerr113
	}

	tagList := make([]tags.Tag, len(tagNames))
	for i, tagName := range tagNames {
		tagList[i] = tags.Tag{Category: tagName, Value: records[0][i]}
	}

	gpu.baseTags = tagList

	return nil
}

// Collect starts the background process if it is not running.
func (gpu *GPU) Collect(ctx context.Context) error {

	gpu.Lock()
	if gpu.running {
		gpu.Unlock()
		return nil
	}

	go func() {
		gpu.running = true
		gpu.Unlock()

		cmd := exec.CommandContext(ctx, gpu.exePath, gpu.exeArgs...) //nolint:gosec

		gpu.logger.Debug().Strs("cmd", cmd.Args).Msg("starting")

		var errOut bytes.Buffer
		cmd.Stderr = &errOut

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			gpu.logger.Error().Err(err).Msg("stdout pipe")
			gpu.Lock()
			gpu.running = false
			gpu.Unlock()
			return
		}

		scanner := bufio.NewScanner(stdout)

		if err := cmd.Start(); err != nil {
			gpu.logger.Error().Err(err).Str("cmd", gpu.exePath).Msg("cmd start")
			gpu.Lock()
			gpu.running = false
			gpu.Unlock()
			return
		}

		for scanner.Scan() {
			err := gpu.parseOutput(scanner.Text())
			if err != nil {
				gpu.logger.Warn().Err(err).Msg("parsing output")
			}
			done := false
			select {
			case <-ctx.Done():
				done = true
			default:
			}
			if done {
				break
			}
		}

		if err := scanner.Err(); err != nil {
			gpu.logger.Error().Err(err).Msg("reading stdio")
		}

		if err := cmd.Wait(); err != nil {
			var stderr string
			if errOut.Len() > 0 {
				stderr = strings.ReplaceAll(errOut.String(), "\n", "")
			}
			var exiterr *exec.ExitError
			if errors.As(err, &exiterr) {
				// if exiterr, ok := err.(*exec.ExitError); ok {
				errMsg := fmt.Sprintf("%s %s", stderr, exiterr.Stderr)
				gpu.logger.Error().
					Str("stderr", errMsg).
					Str("status", exiterr.String()).
					Str("cmd", gpu.exePath).
					Msg("exited non-zero")
			} else {
				gpu.logger.Error().
					Err(err).
					Str("cmd", gpu.exePath).
					Str("stderr", stderr).
					Msg("exited non-zero (not exiterr)")
			}
		}

		gpu.Lock()
		gpu.running = false
		gpu.Unlock()
	}()

	return nil
}

func (gpu *GPU) parseOutput(line string) error {
	if line == "" {
		return nil
	}

	r := csv.NewReader(strings.NewReader(line))

	records, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("parsing csv: %w", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("no metrics found in line (%s)", line) //nolint:goerr113
	}

	for lineID, record := range records {
		if len(record) == 0 {
			gpu.logger.Warn().Int("line", lineID).Strs("values", record).Msg("no metrics found on line")
			continue
		}
		if len(record) != len(gpu.metricList) {
			gpu.logger.Warn().Int("line", lineID).Strs("values", record).Int("num_values", len(record)).Int("num_metrics", len(gpu.metricList)).Msg("mismatch values != expected metrics")
			continue
		}

		for i, metric := range gpu.metricList {
			metricName := metric.MetricName
			if metricName == "" {
				metricName = metric.ArgName
			}
			origValue := strings.TrimSpace(record[i])
			if metric.MetricType[:1] != "t" && origValue == "Not Available" {
				// gpu.logger.Warn().Str("name", metricName).Str("value", origValue).Msg("ignoring")
				continue
			}
			if metric.MetricType[:1] != "t" && origValue == "N/A" {
				// gpu.logger.Warn().Str("name", metricName).Str("value", origValue).Msg("ignoring")
				continue
			}
			metricType := metric.MetricType
			units := metric.Units
			vmatch := metric.MatchValue
			switch metricType {
			case "h":
				v, err := strconv.ParseFloat(origValue, 64)
				if err != nil {
					gpu.logger.Warn().Err(err).Str("name", metricName).Str("val", origValue).Msg("float64 conversion error")
					continue
				}
				var tagList cgm.Tags
				tagList = append(tagList, gpu.baseTags...)
				if units != "" {
					tagList = append(tagList, cgm.Tag{Category: "units", Value: units})
				}
				gpu.metrics.RecordValueWithTags(metricName, tagList, v)
			case "th1":
				val := float64(0)
				if origValue == vmatch.(string) {
					// gpu.logger.Info().Str("metric", metricName).Msg("matched")
					val = 1.0
				}
				var tagList cgm.Tags
				tagList = append(tagList, gpu.baseTags...)
				if units != "" {
					tagList = append(tagList, cgm.Tag{Category: "units", Value: units})
				}
				// gpu.logger.Info().Str("metric", metricName).Str("orig", origValue).Float64("stat", val).Msg("th1")
				gpu.metrics.RecordValueWithTags(metricName, tagList, val)
			case "th2":
				val := float64(-1)
				for j, v := range vmatch.([]string) {
					if origValue == v {
						val = float64(j)
						break
					}
				}
				var tagList cgm.Tags
				tagList = append(tagList, gpu.baseTags...)
				if units != "" {
					tagList = append(tagList, cgm.Tag{Category: "units", Value: units})
				}
				// gpu.logger.Info().Str("metric", metricName).Str("orig", origValue).Float64("stat", val).Msg("th2")
				gpu.metrics.RecordValueWithTags(metricName, tagList, val)
			default:
				gpu.logger.Warn().Str("type", metricType).Str("name", metricName).Msg("unknown metric type")

			}
		}
	}

	return nil
}
