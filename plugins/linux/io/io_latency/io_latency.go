// Copyright © 2017 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/circonus-labs/circonus-agent/plugins/linux/io/eventharness"
	"github.com/circonus-labs/circonusllhist"
)

var maxAge = 310.0 // 5m10s
var devlist = map[string]string{}

var inserts = map[string]float64{}
var issues = map[string]float64{}
var maxWhence = float64(0)
var latencies = map[string]interface{}{}
var cleantables = time.Ticker{}

func trackLatency(dev, typ string, duration float64) {
	var latency map[string]*circonusllhist.Histogram
	if latencyGen, ok := latencies[typ]; ok {
		latency = latencyGen.(map[string]*circonusllhist.Histogram)
	} else {
		latency = make(map[string]*circonusllhist.Histogram)
		latencies[typ] = latency
	}
	hist, ok2 := latency[dev]
	if !ok2 {
		latency[dev] = circonusllhist.NewNoLocks()
		hist = latency[dev]
	}
	_ = hist.RecordValue(duration)
}

func dumpHistAndClear() {
	tmp := map[string]interface{}{}
	for typ, v := range latencies {
		latency := v.(map[string]*circonusllhist.Histogram)
		for dev, hist := range latency {
			if ss := hist.DecStrings(); len(ss) > 0 {
				hist.Reset()
				val := make(map[string]interface{})
				val["_type"] = "h"
				val["_value"] = ss
				val["_tags"] = []string{"device:"+dev,"units:seconds"}
				tmp[typ] = val
			}
		}
	}
	if buff, err := json.Marshal(tmp); err == nil {
		fmt.Printf("%s\n\n", string(buff))
	}
}

func handleLogLine(line string) {
	parts := strings.Fields(line)
	if len(parts) >= 11 && devlist[parts[5]] != "" {
		whence, err := strconv.ParseFloat(strings.Replace(parts[3], ":", "", 1), 64)
		if err != nil {
			return
		}
		if whence > maxWhence {
			maxWhence = whence
		}
		op, dev := strings.Replace(parts[4], ":", "", 1), devlist[parts[5]]
		if whence == 0 {
			return
		}
		select {
		case <-cleantables.C:
			for k, v := range inserts {
				if maxWhence-v > maxAge {
					delete(inserts, k)
				}
			}
			for k, v := range issues {
				if maxWhence-v > maxAge {
					delete(inserts, k)
				}
			}
		default:
		}

		switch {
		case op == "block_rq_insert":
			sec, nsec := parts[9], parts[11]
			opkey := strings.Join([]string{dev, sec, nsec}, ",")
			inserts[opkey] = whence
		case op == "block_rq_issue":
			sec, nsec := parts[9], parts[11]
			opkey := strings.Join([]string{dev, sec, nsec}, ",")
			if start, ok := inserts[opkey]; ok {
				issues[opkey] = whence
				// Q2D - time request spent in queue - io_latency`queue_time
				trackLatency(dev, "queue_time", whence-start)
			}
		case op == "block_rq_complete":
			sec, nsec := parts[8], parts[10]
			opkey := strings.Join([]string{dev, sec, nsec}, ",")
			if start, ok := inserts[opkey]; ok {
				// Q2C - total request handling time - io_latency`total_time
				trackLatency(dev, "total_time", whence-start)
				delete(inserts, opkey)
			}
			if start, ok := issues[opkey]; ok {
				// D2C - time for device to handle request - io_latency`device_time
				trackLatency(dev, "device_time", whence-start)
				delete(issues, opkey)
			}
		default:
		}
	}
}

func refreshDevs() {
	dir, err := os.Open("/dev/")
	if err != nil {
		return
	}
	files, err := dir.Readdir(0)
	if err != nil {
		return
	}
	for _, file := range files {
		if stat := file.Sys().(*syscall.Stat_t); file.Mode()&os.ModeDevice != 0 && file.Mode()&os.ModeCharDevice == 0 && stat != nil {
			major, minor := stat.Rdev/256, stat.Rdev%256
			key := fmt.Sprintf("%d,%d", major, minor)
			if _, ok := devlist[key]; !ok {
				devlist[key] = file.Name()
			}
		}
	}
}
func cleanupTables() {
	for k, v := range inserts {
		if maxWhence-v > maxAge {
			delete(inserts, k)
		}
	}
	for k, v := range issues {
		if maxWhence-v > maxAge {
			delete(inserts, k)
		}
	}
}
func main() {
	refreshDevs()

	h, err := eventharness.HarnessMain("circ_blk",
		[][]string{{"events/block/block_rq_issue/enable", "1\n"},
			{"events/block/block_rq_insert/enable", "1\n"},
			{"events/block/block_rq_complete/enable", "1\n"}},
		handleLogLine)
	if err != nil {
		log.Fatalf("Cannot start tracing.\n%s\n", err)
	}

	go func() {
		for {
			time.Sleep(10 * time.Second)
			h.Tasks <- cleanupTables
			h.Tasks <- dumpHistAndClear
		}
	}()

	err = <-h.Done
	if err != nil {
		log.Fatalf("runtime error: %s\n", err)
	}
}
